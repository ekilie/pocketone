package apitokens

import (
	"net/http"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

func TestImportExtraPrefix(t *testing.T) {
	app, tk := newApp(t, Config{})
	if err := tk.RegisterPrefix("ph_pat_"); err != nil {
		t.Fatal(err)
	}
	if err := tk.RegisterPrefix("ph_pat_"); err != nil {
		t.Fatalf("registering again: %v", err)
	}
	for _, bad := range []string{"", "ph", "PH_", "ph-pat_", "pbt_x_", "ph_"} {
		if err := tk.RegisterPrefix(bad); err == nil {
			t.Errorf("RegisterPrefix(%q) accepted", bad)
		}
	}
	if got := tk.Prefixes(); len(got) != 2 || got[0] != "pbt_" || got[1] != "ph_pat_" {
		t.Fatalf("Prefixes() = %v", got)
	}
	su := superuser(t, app)
	legacy := "ph_pat_abcdefghijklmno_" + strings.Repeat("Z", 43)

	rec, err := tk.Import(su, legacy, CreateOptions{Name: "ci", Scopes: []string{"deploy"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := rec.GetString("prefix"); got != "ph_pat_abcdef" {
		t.Fatalf("display prefix = %q", got)
	}
	info, err := tk.Verify(legacy)
	if err != nil || info.Token.Id != rec.Id || info.Owner.Id != su.Id {
		t.Fatalf("Verify(imported) = %v, %v", info, err)
	}
	if _, err := tk.Import(su, legacy, CreateOptions{}); err == nil {
		t.Fatal("importing the same secret twice should fail")
	}

	for name, s := range map[string]string{
		"wrong prefix":           "xx_pat_" + strings.Repeat("a", 50),
		"too short after prefix": "ph_pat_" + strings.Repeat("a", 31),
	} {
		if _, err := tk.Import(su, s, CreateOptions{}); err == nil {
			t.Errorf("%s: Import accepted %q", name, s)
		}
	}
	moved := "pbt_" + strings.Repeat("m", 40) // a pbt_ token from another app
	if _, err := tk.Import(su, moved, CreateOptions{}); err != nil {
		t.Fatalf("importing a pbt_ token: %v", err)
	}
	if _, err := tk.Verify(moved); err != nil {
		t.Fatalf("Verify(imported pbt_) = %v", err)
	}
	if !tk.IsSecret(legacy) || tk.IsSecret("ph_pat_short") || !tk.IsSecret("pbt_"+strings.Repeat("a", 40)) {
		t.Fatal("IsSecret")
	}
	if _, err := tk.Verify("ph_pat_abcdefghijklmno_" + strings.Repeat("Y", 43)); err != ErrInvalid {
		t.Fatalf("unknown imported-shape secret: %v", err)
	}

	factory := func(testing.TB) *tests.TestApp { return app }
	for _, s := range []tests.ApiScenario{
		{Name: "an imported token authenticates", Method: http.MethodGet, URL: "/api/deploytest/ping",
			Headers: authHeader(legacy), ExpectedStatus: 200, ExpectedContent: []string{"pong test@example.com"}},
		{Name: "held to its scopes", Method: http.MethodGet, URL: "/api/collections", Headers: authHeader(legacy),
			ExpectedStatus: 403, ExpectedContent: []string{"scopes (deploy)"}},
		{Name: "an unknown secret with the extra prefix is rejected", Method: http.MethodGet,
			URL: "/api/deploytest/ping", Headers: authHeader("ph_pat_abcdefghijklmno_" + strings.Repeat("Y", 43)),
			ExpectedStatus: 401, ExpectedContent: []string{`"status":401`}},
	} {
		s.TestAppFactory = factory
		s.DisableTestAppCleanup = true
		s.Test(t)
	}
}

func TestOnlyPbtByDefault(t *testing.T) {
	app, tk := newApp(t, Config{})
	if _, err := tk.Import(superuser(t, app), "ph_pat_"+strings.Repeat("a", 50), CreateOptions{}); err == nil {
		t.Fatal("Import of an unregistered prefix should fail")
	}
	if tk.IsSecret("ph_pat_" + strings.Repeat("a", 50)) {
		t.Fatal("a ph_pat_ secret isn't one until ph_pat_ is registered")
	}
}
