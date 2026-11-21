package apitokens

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

func TestVerifyContract(t *testing.T) {
	app, tk := newApp(t, Config{})
	tk.MustRegisterScope(Scope{Name: "notes", Description: "Notes", Paths: []string{"/api/notes/"},
		Values: &ScopeValues{Description: "a note id"}})
	su := superuser(t, app)
	jwt, err := su.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	limited, rec, err := tk.Create(su, CreateOptions{Name: "ci", Scopes: []string{"notes=abc"}})
	if err != nil {
		t.Fatal(err)
	}
	full, _, err := tk.Create(su, CreateOptions{Name: "full"})
	if err != nil {
		t.Fatal(err)
	}
	expired, _, err := tk.Create(su, CreateOptions{Expires: time.Now().Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	tk.cfg.Now = func() time.Time { return time.Now().Add(time.Hour) }
	t.Cleanup(func() { tk.cfg.Now = time.Now })

	body := func(token string) string { return `{"token":"` + token + `","ip":"203.0.113.9"}` }
	su1 := map[string]string{"Authorization": jwt}
	factory := func(testing.TB) *tests.TestApp { return app }
	for _, s := range []tests.ApiScenario{
		{Name: "anonymous", Method: http.MethodPost, URL: "/api/_c/api-tokens.verify/v1", Body: stringsReader(body(full)),
			ExpectedStatus: 401, ExpectedContent: []string{`"unauthorized"`}},
		{Name: "a token limited to other scopes can't verify", Method: http.MethodPost,
			URL: "/api/_c/api-tokens.verify/v1", Body: stringsReader(body(full)), Headers: authHeader(limited),
			ExpectedStatus: 403, ExpectedContent: []string{"scopes (notes=abc)"}},
		{Name: "valid, with scope values", Method: http.MethodPost, URL: "/api/_c/api-tokens.verify/v1",
			Body: stringsReader(body(limited)), Headers: su1, ExpectedStatus: 200,
			ExpectedContent: []string{`"valid":true`, `"tokenId":"` + rec.Id + `"`, `"name":"ci"`,
				`"ownerCollection":"_superusers"`, `"ownerId":"` + su.Id + `"`, `"scopes":["notes=abc"]`}},
		{Name: "valid, unscoped", Method: http.MethodPost, URL: "/api/_c/api-tokens.verify/v1",
			Body: stringsReader(body(full)), Headers: authHeader(full), ExpectedStatus: 200,
			ExpectedContent: []string{`"valid":true`, `"scopes":[]`}},
		{Name: "unknown", Method: http.MethodPost, URL: "/api/_c/api-tokens.verify/v1",
			Body: stringsReader(body("pbt_nope")), Headers: su1, ExpectedStatus: 200,
			ExpectedContent: []string{`"valid":false`, `"reason":"invalid"`}, NotExpectedContent: []string{`"grant"`}},
		{Name: "expired", Method: http.MethodPost, URL: "/api/_c/api-tokens.verify/v1",
			Body: stringsReader(body(expired)), Headers: su1, ExpectedStatus: 200,
			ExpectedContent: []string{`"valid":false`, `"reason":"expired"`}},
	} {
		s.TestAppFactory = factory
		s.DisableTestAppCleanup = true
		s.Test(t)
	}

	got, err := tk.Get(rec.Id)
	if err != nil {
		t.Fatal(err)
	}
	if ip := got.GetString("last_ip"); ip != "203.0.113.9" {
		t.Fatalf("last_ip = %q, want the IP verify was given", ip)
	}
}

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }
