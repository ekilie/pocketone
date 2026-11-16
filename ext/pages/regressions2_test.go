package pages

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// __dirname and __filename are absolute, as in Node and 0.x.
func TestModuleDirnameAbsolute(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"_private/d.js": `module.exports = { d: __dirname, f: __filename }`,
		"index.ejs":     `<%= JSON.stringify(require('./_private/d.js')) %>`,
	})
	code, body := get(t, dir, "/", true)
	if code != 200 {
		t.Fatalf("%d %.300q", code, body)
	}
	var got struct{ D, F string }
	if err := json.Unmarshal([]byte(strings.ReplaceAll(body, "&#34;", `"`)), &got); err != nil {
		t.Fatalf("%v: %q", err, body)
	}
	abs, _ := filepath.Abs(dir)
	if got.D != filepath.Join(abs, "_private") || got.F != filepath.Join(abs, "_private", "d.js") {
		t.Fatalf("got %+v, want under %s", got, abs)
	}
}

// .well-known is served at the top; *.pb.js files are a build error and never served.
func TestWellKnownAndHookFiles(t *testing.T) {
	dir := writeTree(t, map[string]string{
		".well-known/security.txt": "Contact: mailto:security@example.com",
		"blog/.well-known/x.txt":   "nested dot folders stay hidden",
		".hidden/x.txt":            "hidden",
		"routes.pb.js":             "routerAdd('GET', '/x', () => {})",
		"index.ejs":                "home",
	})
	cases := map[string]int{
		"/.well-known/security.txt": 200,
		"/blog/.well-known/x.txt":   404,
		"/.hidden/x.txt":            404,
		"/routes.pb.js":             404,
	}
	for p, want := range cases {
		if code, body := get(t, dir, p, true); code != want {
			t.Errorf("%s: %d %.120q, want %d", p, code, body, want)
		}
	}
	_, errs := Build(dir)
	found := false
	for _, e := range errs {
		found = found || strings.Contains(e.Error(), "routes.pb.js")
	}
	if !found {
		t.Errorf("want a build error for routes.pb.js, got %v", errs)
	}
}

// include finds a dotted name with the extension added, and 0.x's /…/_private/… form from the top of pb_pages.
func TestIncludeDottedAndRootPrivate(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"_private/nav.main.ejs":  "nav",
		"blog/_private/card.ejs": "card",
		"_private/product.ejs":   "product",
		"index.ejs":              `<%- include('nav.main') %>|<%- include('/blog/_private/card') %>|<%- include('/_private/product') %>|<%- include('/product') %>`,
		"blog/index.ejs":         `<%- include('card') %>`,
		"_private/data.cfg.json": `{"n":1}`,
		"res.ejs":                `<%= ctx.resolve('data.cfg').n %>`,
	})
	if code, body := get(t, dir, "/", true); code != 200 || body != "nav|card|product|product" {
		t.Fatalf("/: %d %.300q", code, body)
	}
	if code, body := get(t, dir, "/res", true); code != 200 || body != "1" {
		t.Fatalf("/res: %d %.300q", code, body)
	}
}

// A plugin with a 0.x hook name is an error naming the v1 hook; extendContext runs for every plugin before any
// onRequest.
func TestPluginHooks(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"+config.js": `module.exports = { plugins: [
			() => ({ name: 'a', onRequest(ctx) { ctx.data.seen = ctx.fromB } }),
			() => ({ name: 'b', extendContext(ctx) { ctx.fromB = 'b' } }),
		] }`,
		"index.ejs": `<%= data.seen %>`,
	})
	if code, body := get(t, dir, "/", true); code != 200 || body != "b" {
		t.Fatalf("order: %d %.300q", code, body)
	}

	dir = writeTree(t, map[string]string{
		"+config.js": `module.exports = { plugins: [() => ({ name: 'old', onRender() {} })] }`,
		"index.ejs":  `x`,
	})
	if code, body := get(t, dir, "/", true); code != 500 || !strings.Contains(body, "use transform") {
		t.Fatalf("0.x hook: %d %.300q", code, body)
	}
}

// A layout that prints <%- slot %> (0.x) gets the page, not the function source.
func TestLayoutSlotWithoutCall(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"+layout.ejs": `<main><%- slot %></main>`,
		"index.ejs":   `hi`,
	})
	if code, body := get(t, dir, "/", true); code != 200 || body != "<main>hi</main>" {
		t.Fatalf("%d %.300q", code, body)
	}
}

// An Authorization header login wins over the pb_auth cookie.
func TestAuthHeaderBeatsCookie(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"+config.js": `module.exports = { plugins: ['auth'] }`,
		"index.ejs":  `<%= ctx.auth ? ctx.auth.email() : 'none' %>`,
	})
	h, p := newServer(t, dir)
	u1, err := p.app.FindAuthRecordByEmail("users", "test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := p.app.FindAuthRecordByEmail("users", "test2@example.com")
	if err != nil {
		t.Fatal(err)
	}
	t1, _ := u1.NewAuthToken()
	t2, _ := u2.NewAuthToken()
	cookie, _ := json.Marshal(map[string]string{"token": t2})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", t1)
	req.Header.Set("Cookie", "pb_auth="+urlEscape(string(cookie)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	b, _ := io.ReadAll(rec.Result().Body)
	if string(b) != "test@example.com" {
		t.Fatalf("header should win: %d %q", rec.Code, b)
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "pb_auth="+urlEscape(string(cookie)))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	b, _ = io.ReadAll(rec.Result().Body)
	if string(b) != "test2@example.com" {
		t.Fatalf("cookie alone: %d %q", rec.Code, b)
	}
}

func urlEscape(s string) string {
	return strings.NewReplacer("%", "%25", `"`, "%22", ",", "%2C", ";", "%3B", " ", "%20", "{", "%7B", "}", "%7D", ":", "%3A").Replace(s)
}
