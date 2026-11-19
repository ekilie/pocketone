package pages_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pockethost/pocketone/ext/extkit/modetest"
)

// Every PocketPages starter, the way a new user meets it: `pocketone pages create` and the next steps it prints
// (superuser, then serve), then its pages served as a single app and as an instance on a host. Some starters get a
// closer look: an edit to the minimal page shows on reload without a restart, the htmx counter counts, the auth
// starter registers, signs in and guards its account page, and daisyui serves its CSS but not its Tailwind input.
func TestModesStarters(t *testing.T) {
	bin := modetest.Bin(t)
	out, err := modetest.Command(context.Background(), t, t.TempDir(), modetest.Env(t), "pages", "starters", "--dev=false").Output()
	if err != nil {
		t.Fatalf("pages starters (%s): %v\n%s", bin, err, out)
	}
	var kits []string // the kit lines are indented: "  minimal        Minimal starter (default)"
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) > 0 && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
			kits = append(kits, f[0])
		}
	}
	if len(kits) < 7 {
		t.Fatalf("pages starters lists %d kits:\n%s", len(kits), out)
	}

	for _, kit := range kits {
		t.Run(kit, func(t *testing.T) {
			t.Parallel()
			app := createStarter(t, kit)
			modetest.Run(t, modetest.Topology{Instances: []modetest.Inst{{ID: "myapp", From: app}}}, modetest.AllApp, func(t *testing.T, w *modetest.World) {
				s := w.Instance("myapp")
				page := pageGet(t, s, "/", nil)
				if page.code != 200 || !strings.Contains(page.ctype, "text/html") || !strings.Contains(strings.ToLower(page.body), "<html") {
					t.Fatalf("GET /: %d %s\n%.500s", page.code, page.ctype, page.body)
				}
				switch kit {
				case "minimal":
					minimalEdit(t, s)
				case "htmx":
					for _, want := range []string{"<b>2</b>", "<b>3</b>"} {
						if got := pageGet(t, s, "/fragments/count", nil); got.code != 200 || strings.TrimSpace(got.body) != want {
							t.Fatalf("GET /fragments/count: %d %q, want %s", got.code, got.body, want)
						}
					}
				case "auth":
					authFlow(t, s)
				case "daisyui":
					if got := pageGet(t, s, "/app.css", nil); got.code != 200 || !strings.Contains(got.ctype, "text/css") {
						t.Fatalf("GET /app.css: %d %s", got.code, got.ctype)
					}
					if got := pageGet(t, s, "/-app.tailwind.css", nil); got.code != 404 {
						t.Fatalf("GET /-app.tailwind.css: %d, want 404 (the Tailwind input isn't served)", got.code)
					}
				}
			})
		})
	}
}

// createStarter runs `pages create`, checks the next steps it prints, and returns the app folder it made.
func createStarter(t *testing.T, kit string) string {
	t.Helper()
	base := t.TempDir()
	out, err := modetest.Command(context.Background(), t, base, modetest.Env(t), "pages", "create", "myapp", "--starter", kit, "--dev=false").CombinedOutput()
	if err != nil {
		t.Fatalf("pages create --starter %s: %v\n%s", kit, err, out)
	}
	for _, want := range []string{"cd myapp", "superuser upsert", "serve --dir ./pb_data"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("pages create: no %q in the next steps:\n%s", want, out)
		}
	}
	if strings.Index(string(out), "superuser upsert") > strings.Index(string(out), "serve --dir") {
		t.Fatalf("pages create: serve comes before the superuser step:\n%s", out)
	}
	app := filepath.Join(base, "myapp")
	if _, err := os.Stat(filepath.Join(app, "pb_pages")); err != nil {
		t.Fatal(err)
	}
	return app
}

// minimalEdit: in dev mode an edit to a page shows on the next request, with no restart.
func minimalEdit(t *testing.T, s *modetest.Site) {
	t.Helper()
	index := filepath.Join(s.Dir, "pb_pages", "index.ets")
	b, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(b), "<title>", "<title>Edited <%= 6 * 7 %> ", 1)
	if edited == string(b) {
		t.Fatal("the minimal page has no <title> to edit")
	}
	modetest.WriteFile(t, index, edited)
	modetest.WaitFor(t, "the edited page", 10*time.Second, func() (bool, string) {
		got := pageGet(t, s, "/", nil)
		return strings.Contains(got.body, "<title>Edited 42 "), fmt.Sprintf("%d %.120s", got.code, got.body)
	})
	if s.Restarted() {
		t.Fatalf("the app restarted for a page edit (%d restarts)", s.Restarts())
	}
}

// authFlow: the account page sends a visitor to sign in; registering signs them in (a pb_auth cookie) and the
// account page greets them; signing in again with the password works, and a wrong password is shown on the page.
func authFlow(t *testing.T, s *modetest.Site) {
	t.Helper()
	if got := pageGet(t, s, "/account", nil); got.code/100 != 3 || !strings.HasPrefix(got.location, "/auth/login") {
		t.Fatalf("GET /account signed out: %d → %q", got.code, got.location)
	}
	if got := pageGet(t, s, "/auth/login", nil); got.code != 200 || !strings.Contains(got.body, `name="password"`) {
		t.Fatalf("GET /auth/login: %d\n%.500s", got.code, got.body)
	}

	email, password := "reader@example.com", "reader-password-1"
	reg := pagePost(t, s, "/auth/register", url.Values{"identity": {email}, "password": {password}}, nil)
	cookie := reg.cookie("pb_auth")
	if reg.code/100 != 3 || cookie == nil {
		t.Fatalf("POST /auth/register: %d → %q, cookies %v\n%.500s", reg.code, reg.location, reg.cookies, reg.body)
	}
	if got := pageGet(t, s, "/account", cookie); got.code != 200 || !strings.Contains(got.body, "Welcome, "+email) {
		t.Fatalf("GET /account signed in: %d\n%.500s", got.code, got.body)
	}

	login := pagePost(t, s, "/auth/login", url.Values{"mode": {"password"}, "identity": {email}, "password": {password}}, nil)
	if login.code/100 != 3 || login.cookie("pb_auth") == nil {
		t.Fatalf("POST /auth/login: %d → %q\n%.500s", login.code, login.location, login.body)
	}
	bad := pagePost(t, s, "/auth/login", url.Values{"mode": {"password"}, "identity": {email}, "password": {"wrong-password"}}, nil)
	if bad.code != 200 || bad.cookie("pb_auth") != nil || !strings.Contains(bad.body, "<mark>") {
		t.Fatalf("POST /auth/login with a wrong password: %d, cookies %v\n%.800s", bad.code, bad.cookies, bad.body)
	}
}

// pageAnswer is one response from a pages site, redirects not followed.
type pageAnswer struct {
	code     int
	ctype    string
	location string
	body     string
	cookies  []*http.Cookie
}

func (a pageAnswer) cookie(name string) *http.Cookie {
	for _, c := range a.cookies {
		if c.Name == name && c.Value != "" {
			return c
		}
	}
	return nil
}

func pageGet(t *testing.T, s *modetest.Site, path string, cookie *http.Cookie) pageAnswer {
	t.Helper()
	return pageDo(t, s, http.MethodGet, path, nil, cookie)
}

func pagePost(t *testing.T, s *modetest.Site, path string, form url.Values, cookie *http.Cookie) pageAnswer {
	t.Helper()
	return pageDo(t, s, http.MethodPost, path, form, cookie)
}

func pageDo(t *testing.T, s *modetest.Site, method, path string, form url.Values, cookie *http.Cookie) pageAnswer {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, s.URL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := s.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return pageAnswer{code: res.StatusCode, ctype: res.Header.Get("Content-Type"), location: res.Header.Get("Location"), body: string(b), cookies: res.Cookies()}
}
