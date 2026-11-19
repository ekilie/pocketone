package apitokens

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"

	templatesv1 "github.com/pockethost/pocketone/ext/contracts/templates/v1"
	"github.com/pockethost/pocketone/ext/remotes"
	"github.com/pockethost/pocketone/ext/templates"
)

// The OAuth pages are the "oauth" template set: ejected through templates/v1 into pb_templates/oauth, the project's
// edits render on the next request (the whole sign-in flow keeps working), and deleting them brings back the
// built-in pages.
func TestOAuthPagesThemedThroughTemplates(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	dir := t.TempDir()
	if err := templates.New(templates.Config{Dir: dir}).Register(app, nil); err != nil {
		t.Fatal(err)
	}
	MustRegister(app, Config{OAuth: OAuthConfig{Branding: Branding{Name: "Acme Cloud"}}})
	remotes.MustRegister(app, remotes.Config{})
	if err := app.OnBootstrap().Trigger(&core.BootstrapEvent{App: app}, func(*core.BootstrapEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	EnsureSchema(app)

	tpl := templatesv1.Contract.Must(app)
	written, err := tpl.Eject(TemplateSet, false)
	if err != nil || len(written) != 10 {
		t.Fatalf("eject: %v %v", written, err)
	}
	if written, err := tpl.Eject(TemplateSet, false); err != nil || len(written) != 0 {
		t.Fatalf("second eject of an unchanged copy: %v %v", written, err)
	}
	signin := filepath.Join(dir, "oauth", "signin.tpl")
	b, _ := os.ReadFile(signin)
	if !strings.Contains(string(b), "<h1>Sign in to {{.AppName}}</h1>") {
		t.Fatalf("ejected signin.tpl isn't the built-in Go template:\n%s", b)
	}
	os.WriteFile(signin, []byte(strings.Replace(string(b), "<h1>Sign in to", "<h1>EJECTED Sign in to", 1)), 0o644)

	base := serveApp(t, app)
	_, challenge := pkce()
	q := url.Values{"response_type": {"code"}, "client_id": {CLIClientId}, "redirect_uri": {"http://127.0.0.1:9/cb"}, "state": {"s"},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}}
	br := newBrowser(t)
	br.get(base + "/api/_c/oauth/v1/authorize?" + q.Encode())
	if br.status != 200 || !br.has("EJECTED Sign in to Acme Cloud") || !br.has(`name="csrf"`) {
		t.Fatalf("ejected sign-in: %d %s", br.status, br.page)
	}
	br.submit("password", map[string]string{"collection": "_superusers", "email": "test@example.com", "password": "wrong"})
	if br.status != 401 || !br.has("Wrong email or password") {
		t.Fatalf("ejected error status: %d %s", br.status, br.page)
	}
	br.submit("password", map[string]string{"collection": "_superusers", "email": "test@example.com", "password": "1234567890"})
	if !br.has("wants to access Acme Cloud") || !br.has("Full access") {
		t.Fatalf("consent after the ejected sign-in: %s", br.page)
	}
	res := br.submit("allow", nil)
	if loc := res.Header.Get("Location"); res.StatusCode != 302 || !strings.Contains(loc, "code=") {
		t.Fatalf("allow: %d %s", res.StatusCode, loc)
	}

	// deleting the copy goes back to the built-in pages
	os.RemoveAll(filepath.Join(dir, "oauth"))
	br2 := newBrowser(t)
	br2.get(base + "/api/_c/oauth/v1/authorize?" + q.Encode())
	if !br2.has("Sign in to Acme Cloud") || br2.has("EJECTED") {
		t.Fatalf("built-in after delete: %s", br2.page)
	}
}
