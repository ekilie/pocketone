package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// Mail on one stock app in dev mode, caught by the dev mail trap: a bulk campaign to the users collection (templated
// per recipient, a suppressed address skipped, never twice to anyone), a test send, the one-click unsubscribe link
// from a delivered message, and PocketBase's own mail (password reset) landing in the same trap.
func TestE2EBulkmailToMailTrap(t *testing.T) {
	s := startSite(t, siteOptions{})
	su := s.JWT()
	s.mustJSON(http.MethodPatch, "/api/settings", su, map[string]any{"meta": map[string]any{"appURL": s.URL}}, 200, nil)
	clearTrap(t, s)

	for _, u := range []struct{ email, name string }{{"ann@example.test", "Ann"}, {"bob@example.test", "Bob"}, {"cat@example.test", "Cat"}} {
		s.mustJSON(http.MethodPost, "/api/collections/users/records", su, map[string]any{
			"email": u.email, "name": u.name, "password": "password1234", "passwordConfirm": "password1234"}, 200, nil)
	}
	tok := s.Token()
	s.mustJSON(http.MethodPost, "/api/_c/mail.suppression/v1/suppress", tok,
		map[string]string{"email": "cat@example.test", "reason": "manual"}, 200, nil)

	audience := map[string]any{"collection": "users", "filter": "", "email_field": "email"}
	var count struct {
		Count      int `json:"count"`
		Suppressed int `json:"suppressed"`
	}
	s.mustJSON(http.MethodPost, "/api/_c/bulkmail/v1/count", tok, map[string]any{"audience": audience}, 200, &count)
	if count.Count != 2 || count.Suppressed != 1 { // count is who gets it: suppressed addresses are left out
		t.Fatalf("count: %+v", count)
	}

	campaign := newCampaign(t, s, "Launch for {{record.name}}", `<p>Hi {{record.name}}, this went to {{email}}.</p>`, audience)
	s.mustJSON(http.MethodPost, "/api/_c/bulkmail/v1/start", tok, map[string]any{"campaign": campaign}, 200, nil)
	c := waitCampaign(t, s, campaign, "done")
	if c.Sent != 2 || c.Skipped != 1 || c.Failed != 0 {
		t.Fatalf("campaign: %+v", c)
	}

	msgs := waitTrap(t, s, 2)
	var got []string
	for _, m := range msgs {
		got = append(got, strings.Join(m.RcptTo, ",")+" "+m.Subject)
	}
	sort.Strings(got)
	if strings.Join(got, "|") != "ann@example.test Launch for Ann|bob@example.test Launch for Bob" {
		t.Fatalf("trapped: %q", got)
	}
	ann := trapMessage(t, s, findTo(t, msgs, "ann@example.test"))
	if !strings.Contains(ann.HTML, "Hi Ann, this went to ann@example.test.") {
		t.Fatalf("ann's body: %s", ann.HTML)
	}

	// starting it again sends nobody anything new
	s.raw(http.MethodPost, "/api/_c/bulkmail/v1/start", tok, map[string]any{"campaign": campaign})
	time.Sleep(1500 * time.Millisecond)
	if n := len(listTrap(t, s)); n != 2 {
		t.Fatalf("after starting again: %d trapped, want 2", n)
	}

	// a test send goes to the address given, whatever the audience
	s.mustJSON(http.MethodPost, "/api/_c/bulkmail/v1/test", tok, map[string]any{"campaign": campaign, "to": "qa@example.test"}, 200, nil)
	findTo(t, waitTrap(t, s, 3), "qa@example.test")

	// the one-click unsubscribe from bob's message suppresses bob
	raw := trapRaw(t, s, findTo(t, msgs, "bob@example.test"))
	unsub := listUnsubscribeURL(t, raw)
	if !strings.HasPrefix(unsub, s.URL) {
		t.Fatalf("List-Unsubscribe %q doesn't point at the app (%s)", unsub, s.URL)
	}
	req, _ := http.NewRequest(http.MethodPost, unsub, strings.NewReader("List-Unsubscribe=One-Click"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode >= 300 {
		t.Fatalf("one-click unsubscribe: %d", res.StatusCode)
	}
	var check struct {
		Suppressed bool `json:"suppressed"`
	}
	s.mustJSON(http.MethodGet, "/api/_c/mail.suppression/v1/check?email="+url.QueryEscape("bob@example.test"), tok, nil, 200, &check)
	if !check.Suppressed {
		t.Fatal("bob isn't suppressed after the one-click unsubscribe")
	}

	// a second campaign now reaches only ann
	clearTrap(t, s)
	second := newCampaign(t, s, "Second", "<p>again</p>", audience)
	s.mustJSON(http.MethodPost, "/api/_c/bulkmail/v1/start", tok, map[string]any{"campaign": second}, 200, nil)
	if c := waitCampaign(t, s, second, "done"); c.Sent != 1 || c.Skipped != 2 {
		t.Fatalf("second campaign: %+v", c)
	}
	if msgs := waitTrap(t, s, 1); strings.Join(msgs[0].RcptTo, ",") != "ann@example.test" {
		t.Fatalf("second campaign went to %v", msgs[0].RcptTo)
	}

	// PocketBase's own mail lands in the same trap
	clearTrap(t, s)
	s.mustJSON(http.MethodPost, "/api/collections/users/request-password-reset", "", map[string]string{"email": "ann@example.test"}, 204, nil)
	if m := waitTrap(t, s, 1); strings.Join(m[0].RcptTo, ",") != "ann@example.test" || !strings.Contains(strings.ToLower(m[0].Subject), "reset") {
		t.Fatalf("password reset mail: %+v", m[0])
	}
}

type campaignState struct {
	Status  string `json:"status"`
	Sent    int    `json:"sent"`
	Skipped int    `json:"skipped"`
	Failed  int    `json:"failed"`
}

func newCampaign(t *testing.T, s *site, subject, html string, audience any) string {
	t.Helper()
	var rec struct {
		Id string `json:"id"`
	}
	s.mustJSON(http.MethodPost, "/api/collections/_bulkmail_campaigns/records", s.JWT(), map[string]any{
		"name": subject, "subject": subject, "html": html, "audience": audience, "status": "draft",
		"from_name": "E2E", "from_address": "news@example.test",
	}, 200, &rec)
	return rec.Id
}

func waitCampaign(t *testing.T, s *site, id, status string) campaignState {
	t.Helper()
	var c campaignState
	waitFor(t, context.Background(), "campaign "+status, 20*time.Second, func() (bool, string) {
		code, b := s.raw(http.MethodGet, "/api/collections/_bulkmail_campaigns/records/"+id, s.JWT(), nil)
		if code != 200 {
			return false, fmt.Sprintf("%d %s", code, b)
		}
		_ = json.Unmarshal(b, &c)
		return c.Status == status, fmt.Sprintf("%+v", c)
	})
	return c
}

type trapped struct {
	ID      int      `json:"id"`
	RcptTo  []string `json:"rcptTo"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text"`
}

func listTrap(t *testing.T, s *site) []trapped {
	t.Helper()
	code, b := rawRequest(s.TrapURL, http.MethodGet, "/api/messages", "", nil)
	if code != 200 {
		t.Fatalf("mail trap inbox: %d %s", code, b)
	}
	var out []trapped
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("mail trap inbox: %v %s", err, b)
	}
	return out
}

func waitTrap(t *testing.T, s *site, n int) []trapped {
	t.Helper()
	var msgs []trapped
	waitFor(t, context.Background(), fmt.Sprintf("%d messages in the mail trap", n), 15*time.Second, func() (bool, string) {
		msgs = listTrap(t, s)
		return len(msgs) >= n, fmt.Sprintf("%d messages", len(msgs))
	})
	if len(msgs) != n {
		t.Fatalf("mail trap: %d messages, want %d: %+v", len(msgs), n, msgs)
	}
	return msgs
}

func clearTrap(t *testing.T, s *site) {
	t.Helper()
	if code, b := rawRequest(s.TrapURL, http.MethodDelete, "/api/messages", "", nil); code != 204 {
		t.Fatalf("clear mail trap: %d %s", code, b)
	}
}

func findTo(t *testing.T, msgs []trapped, to string) int {
	t.Helper()
	for _, m := range msgs {
		for _, r := range m.RcptTo {
			if r == to {
				return m.ID
			}
		}
	}
	t.Fatalf("no message to %s in %+v", to, msgs)
	return 0
}

func trapMessage(t *testing.T, s *site, id int) trapped {
	t.Helper()
	code, b := rawRequest(s.TrapURL, http.MethodGet, fmt.Sprintf("/api/messages/%d", id), "", nil)
	var m trapped
	if code != 200 || json.Unmarshal(b, &m) != nil {
		t.Fatalf("trap message %d: %d %s", id, code, b)
	}
	return m
}

func trapRaw(t *testing.T, s *site, id int) string {
	t.Helper()
	code, b := rawRequest(s.TrapURL, http.MethodGet, fmt.Sprintf("/api/messages/%d/raw", id), "", nil)
	if code != 200 {
		t.Fatalf("trap raw %d: %d %s", id, code, b)
	}
	return string(b)
}

var listUnsubRE = regexp.MustCompile(`(?mi)^List-Unsubscribe:\s*(?:.*?)<(https?://[^>]+)>`)

// listUnsubscribeURL reads the https URL from a raw message's List-Unsubscribe header (unfolding continuation lines).
func listUnsubscribeURL(t *testing.T, raw string) string {
	t.Helper()
	unfolded := regexp.MustCompile(`\r?\n[ \t]+`).ReplaceAllString(raw, " ")
	m := listUnsubRE.FindStringSubmatch(unfolded)
	if m == nil {
		t.Fatalf("no List-Unsubscribe URL in:\n%s", tail(raw, 40))
	}
	return m[1]
}
