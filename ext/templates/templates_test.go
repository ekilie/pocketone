package templates

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"

	templatesv1 "github.com/pockethost/pocketone/ext/contracts/templates/v1"
)

var set = fstest.MapFS{
	"parts/head.tpl": {Data: []byte(`<h1>{{.Title}}</h1>`)},
	"signin.tpl":     {Data: []byte(`{{template "parts/head" .}}<form>{{.Field}}</form>`)},
}

func render(t *testing.T, p *provider, page string, data any) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	e := &core.RequestEvent{App: p.app}
	e.Response, e.Request = rec, httptest.NewRequest(http.MethodGet, "/", nil)
	if err := p.Render(e, "oauth", page, 0, data); err != nil {
		t.Fatal(err)
	}
	return rec.Code, rec.Body.String()
}

// The built-in set renders with its pieces; a project copy of one file replaces just that file and is read on the next
// render; eject copies the whole set, refusing to overwrite without force.
func TestRenderAndEject(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	templatesv1.Add(app, "oauth", set)
	p := &provider{app: app, dir: t.TempDir()}
	data := map[string]any{"Title": "Sign in", "Field": "<b>x</b>"}

	if code, body := render(t, p, "signin", data); code != 200 || body != `<h1>Sign in</h1><form>&lt;b&gt;x&lt;/b&gt;</form>` {
		t.Fatalf("built in: %d %q", code, body)
	}

	written, err := p.Eject("oauth", false)
	if err != nil || len(written) != 2 {
		t.Fatalf("eject: %v %v", written, err)
	}
	head := filepath.Join(p.dir, "oauth", "parts", "head.tpl")
	if b, _ := os.ReadFile(head); string(b) != `<h1>{{.Title}}</h1>` {
		t.Fatalf("ejected head: %q", b)
	}
	// nothing changed: a second eject writes nothing and asks nothing
	if written, err := p.Eject("oauth", false); err != nil || len(written) != 0 {
		t.Fatalf("second eject: %v %v", written, err)
	}

	// edit one piece, drop the page: the page falls back to the built-in file, which includes the edited piece
	os.WriteFile(head, []byte(`<h1 class="themed">{{.Title}}</h1>`), 0o644)
	os.Remove(filepath.Join(p.dir, "oauth", "signin.tpl"))
	if _, body := render(t, p, "signin", data); !strings.HasPrefix(body, `<h1 class="themed">Sign in</h1><form>`) {
		t.Fatalf("themed: %q", body)
	}

	// a changed file blocks a plain eject (nothing written, not even the missing page); force restores the set
	var ee *templatesv1.ExistsError
	if _, err := p.Eject("oauth", false); !errors.As(err, &ee) || len(ee.Files) != 1 || ee.Files[0] != head {
		t.Fatalf("eject over a change: %v", err)
	}
	if _, err := os.Stat(filepath.Join(p.dir, "oauth", "signin.tpl")); !os.IsNotExist(err) {
		t.Fatal("a refused eject wrote the missing file")
	}
	if written, err := p.ejectMissing("oauth"); err != nil || len(written) != 1 || !strings.HasSuffix(written[0], "signin.tpl") {
		t.Fatalf("eject keeping changes: %v %v", written, err)
	}
	if b, _ := os.ReadFile(head); !strings.Contains(string(b), "themed") {
		t.Fatal("keeping changes overwrote them")
	}
	if written, err := p.Eject("oauth", true); err != nil || len(written) != 1 {
		t.Fatalf("force: %v %v", written, err)
	}
	if b, _ := os.ReadFile(head); strings.Contains(string(b), "themed") {
		t.Fatal("force didn't restore the built-in file")
	}
	if _, err := p.Eject("nope", false); err == nil || !strings.Contains(err.Error(), "oauth") {
		t.Fatalf("unknown set: %v", err)
	}
}

// A broken project copy is an error, not a silent fallback, so the project sees what to fix.
func TestBrokenCopyIsAnError(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	templatesv1.Add(app, "oauth", set)
	p := &provider{app: app, dir: t.TempDir()}
	os.MkdirAll(filepath.Join(p.dir, "oauth"), 0o755)
	os.WriteFile(filepath.Join(p.dir, "oauth", "signin.tpl"), []byte(`{{if}}`), 0o644)
	e := &core.RequestEvent{App: app}
	e.Response, e.Request = httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
	if err := p.Render(e, "oauth", "signin", 0, nil); err == nil || !strings.Contains(err.Error(), "signin.tpl") {
		t.Fatalf("broken copy: %v", err)
	}
}

// The command: list shows the sets and what changed; without a terminal it won't overwrite changes, with --force it
// does.
func TestCommand(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	templatesv1.Add(app, "oauth", set)
	p := &provider{app: app, dir: t.TempDir()}
	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		cmd := NewCommand(p)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetIn(strings.NewReader("y\n")) // not a terminal: never read
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	if out, err := run("list"); err != nil || !strings.Contains(out, "oauth") || !strings.Contains(out, "built in (2 files)") {
		t.Fatalf("list: %v\n%s", err, out)
	}
	if out, err := run("eject"); err != nil || !strings.Contains(out, "oauth: 2 file(s) written") {
		t.Fatalf("eject all: %v\n%s", err, out)
	}
	os.WriteFile(filepath.Join(p.dir, "oauth", "signin.tpl"), []byte("mine"), 0o644)
	if out, err := run("list"); err != nil || !strings.Contains(out, "1 changed, 1 unchanged, 0 missing") {
		t.Fatalf("list after a change: %v\n%s", err, out)
	}
	if out, err := run("eject", "oauth"); err == nil || !strings.Contains(err.Error(), "--force") || !strings.Contains(out, "signin.tpl") {
		t.Fatalf("eject over a change without a terminal: %v\n%s", err, out)
	}
	if b, _ := os.ReadFile(filepath.Join(p.dir, "oauth", "signin.tpl")); string(b) != "mine" {
		t.Fatal("the change was overwritten")
	}
	if out, err := run("eject", "oauth", "--force"); err != nil || !strings.Contains(out, "1 file(s) written") {
		t.Fatalf("force: %v\n%s", err, out)
	}
}
