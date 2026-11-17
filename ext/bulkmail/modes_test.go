package bulkmail_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pockethost/pocketone/ext/extkit/modetest"
)

// Bulk mail on the real binary, as a single app and as an instance on a host, caught by the dev mail trap: a campaign
// to the users collection (templated per recipient, a suppressed address skipped, never twice to anyone), a test send,
// the one-click unsubscribe link from a delivered message, and PocketBase's own mail (password reset) in the same
// trap. A second instance shows the suppression list is the instance's own.
func TestModes(t *testing.T) {
	top := modetest.Topology{Instances: []modetest.Inst{{ID: "news"}, {ID: "other"}}}
	modetest.Run(t, top, []modetest.Mode{modetest.Single, modetest.Host}, func(t *testing.T, w *modetest.World) {
		s, other := w.Instance("news"), w.Instance("other")
		trap := s.Trap()
		su := s.JWT()
		s.MustJSON(http.MethodPatch, "/api/settings", su, map[string]any{"meta": map[string]any{"appURL": s.URL}}, 200, nil)
		trap.Clear()

		for _, u := range []struct{ email, name string }{{"ann@example.test", "Ann"}, {"bob@example.test", "Bob"}, {"cat@example.test", "Cat"}} {
			s.MustJSON(http.MethodPost, "/api/collections/users/records", su, map[string]any{
				"email": u.email, "name": u.name, "password": "password1234", "passwordConfirm": "password1234"}, 200, nil)
		}
		tok := s.Token()
		s.MustJSON(http.MethodPost, "/api/_c/mail.suppression/v1/suppress", tok,
			map[string]string{"email": "cat@example.test", "reason": "manual"}, 200, nil)

		audience := map[string]any{"collection": "users", "filter": "", "email_field": "email"}
		var count struct {
			Count      int `json:"count"`
			Suppressed int `json:"suppressed"`
		}
		s.MustJSON(http.MethodPost, "/api/_c/bulkmail/v1/count", tok, map[string]any{"audience": audience}, 200, &count)
		if count.Count != 2 || count.Suppressed != 1 { // count is who gets it: suppressed addresses are left out
			t.Fatalf("count: %+v", count)
		}

		campaign := newCampaign(t, s, "Launch for {{record.name}}", `<p>Hi {{record.name}}, this went to {{email}}.</p>`, audience)
		s.MustJSON(http.MethodPost, "/api/_c/bulkmail/v1/start", tok, map[string]any{"campaign": campaign}, 200, nil)
		if c := waitCampaign(t, s, campaign, "done"); c.Sent != 2 || c.Skipped != 1 || c.Failed != 0 {
			t.Fatalf("campaign: %+v", c)
		}

		msgs := trap.Wait(2)
		var got []string
		for _, m := range msgs {
			got = append(got, strings.Join(m.RcptTo, ",")+" "+m.Subject)
		}
		sort.Strings(got)
		if strings.Join(got, "|") != "ann@example.test Launch for Ann|bob@example.test Launch for Bob" {
			t.Fatalf("trapped: %q", got)
		}
		if ann := trap.Message(trap.To(msgs, "ann@example.test")); !strings.Contains(ann.HTML, "Hi Ann, this went to ann@example.test.") {
			t.Fatalf("ann's body: %s", ann.HTML)
		}

		// starting it again sends nobody anything new
		s.Raw(http.MethodPost, "/api/_c/bulkmail/v1/start", tok, map[string]any{"campaign": campaign})
		time.Sleep(1500 * time.Millisecond)
		if n := len(trap.List()); n != 2 {
			t.Fatalf("after starting again: %d trapped, want 2", n)
		}

		// a test send goes to the address given, whatever the audience
		s.MustJSON(http.MethodPost, "/api/_c/bulkmail/v1/test", tok, map[string]any{"campaign": campaign, "to": "qa@example.test"}, 200, nil)
		trap.To(trap.Wait(3), "qa@example.test")

		// the one-click unsubscribe from bob's message suppresses bob, on this instance only
		unsub := listUnsubscribeURL(t, trap.Raw(trap.To(msgs, "bob@example.test")))
		if !strings.HasPrefix(unsub, s.URL) {
			t.Fatalf("List-Unsubscribe %q doesn't point at the app (%s)", unsub, s.URL)
		}
		req, _ := http.NewRequest(http.MethodPost, unsub, strings.NewReader("List-Unsubscribe=One-Click"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := s.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode >= 300 {
			t.Fatalf("one-click unsubscribe: %d", res.StatusCode)
		}
		if !suppressed(t, s, tok, "bob@example.test") {
			t.Fatal("bob isn't suppressed after the one-click unsubscribe")
		}
		if suppressed(t, other, other.Token(), "bob@example.test") {
			t.Fatal("bob's unsubscribe from news suppressed him on another instance")
		}

		// a second campaign now reaches only ann
		trap.Clear()
		second := newCampaign(t, s, "Second", "<p>again</p>", audience)
		s.MustJSON(http.MethodPost, "/api/_c/bulkmail/v1/start", tok, map[string]any{"campaign": second}, 200, nil)
		if c := waitCampaign(t, s, second, "done"); c.Sent != 1 || c.Skipped != 2 {
			t.Fatalf("second campaign: %+v", c)
		}
		if msgs := trap.Wait(1); strings.Join(msgs[0].RcptTo, ",") != "ann@example.test" {
			t.Fatalf("second campaign went to %v", msgs[0].RcptTo)
		}

		// PocketBase's own mail lands in the same trap
		trap.Clear()
		s.MustJSON(http.MethodPost, "/api/collections/users/request-password-reset", "", map[string]string{"email": "ann@example.test"}, 204, nil)
		if m := trap.Wait(1); strings.Join(m[0].RcptTo, ",") != "ann@example.test" || !strings.Contains(strings.ToLower(m[0].Subject), "reset") {
			t.Fatalf("password reset mail: %+v", m[0])
		}
	})
}

type campaignState struct {
	Status  string `json:"status"`
	Sent    int    `json:"sent"`
	Skipped int    `json:"skipped"`
	Failed  int    `json:"failed"`
}

func newCampaign(t *testing.T, s *modetest.Site, subject, html string, audience any) string {
	t.Helper()
	var rec struct {
		Id string `json:"id"`
	}
	s.MustJSON(http.MethodPost, "/api/collections/_bulkmail_campaigns/records", s.JWT(), map[string]any{
		"name": subject, "subject": subject, "html": html, "audience": audience, "status": "draft",
		"from_name": "Mode test", "from_address": "news@example.test",
	}, 200, &rec)
	return rec.Id
}

func waitCampaign(t *testing.T, s *modetest.Site, id, status string) campaignState {
	t.Helper()
	var c campaignState
	modetest.WaitFor(t, "campaign "+status, 20*time.Second, func() (bool, string) {
		code, b := s.Raw(http.MethodGet, "/api/collections/_bulkmail_campaigns/records/"+id, s.JWT(), nil)
		if code != 200 {
			return false, fmt.Sprintf("%d %s", code, b)
		}
		_ = json.Unmarshal(b, &c)
		return c.Status == status, fmt.Sprintf("%+v", c)
	})
	return c
}

func suppressed(t *testing.T, s *modetest.Site, tok, email string) bool {
	t.Helper()
	var check struct {
		Suppressed bool `json:"suppressed"`
	}
	s.MustJSON(http.MethodGet, "/api/_c/mail.suppression/v1/check?email="+url.QueryEscape(email), tok, nil, 200, &check)
	return check.Suppressed
}

var listUnsubRE = regexp.MustCompile(`(?mi)^List-Unsubscribe:\s*(?:.*?)<(https?://[^>]+)>`)

// listUnsubscribeURL reads the URL from a raw message's List-Unsubscribe header (unfolding continuation lines).
func listUnsubscribeURL(t *testing.T, raw string) string {
	t.Helper()
	unfolded := regexp.MustCompile(`\r?\n[ \t]+`).ReplaceAllString(raw, " ")
	m := listUnsubRE.FindStringSubmatch(unfolded)
	if m == nil {
		t.Fatalf("no List-Unsubscribe URL in:\n%s", modetest.Tail(raw, 40))
	}
	return m[1]
}
