package pages

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPageJSWithoutTemplate(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"+middleware.js":     "module.exports = (ctx, next) => { ctx.seen = 'mw'; return next() }",
		"logout/+page.js":    "module.exports = { post(ctx) { ctx.res.redirect('/') } }",
		"hello/+page.js":     "module.exports = { get(ctx) { ctx.res.html('<p>hi from js, ' + ctx.seen + '</p>') } }",
		"users/[x]/+page.js": "module.exports = { get(ctx) { ctx.res.html('user ' + ctx.params.x) } }",
		"silent/+page.js":    "module.exports = { get(ctx) {} }",
		"boom/+page.js":      "module.exports = { get(ctx) { throw new Error('kaboom') } }",
		// a folder with a page: +page.js stays that page's handler, and the folder URL isn't routed
		"shop/about.ejs": "about <%= data.n %>",
		"shop/+page.js":  "module.exports = { load() { return { n: 1 } } }",
		// +api.js wins over +page.js in a folder with no page
		"feed/+api.js":  "module.exports = { get(ctx) { ctx.res.json({ api: true }) } }",
		"feed/+page.js": "module.exports = { get(ctx) { ctx.res.html('page') } }",
	})
	h, _ := newServer(t, dir)
	do := func(method, p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, p, nil))
		return rec
	}

	if rec := do("POST", "/logout"); rec.Code != 302 || rec.Header().Get("Location") != "/" {
		t.Fatalf("POST /logout: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := do("GET", "/logout"); rec.Code != 405 || rec.Header().Get("Allow") != "POST" {
		t.Fatalf("GET /logout: %d Allow %q", rec.Code, rec.Header().Get("Allow"))
	}
	if rec := do("GET", "/hello"); rec.Code != 200 || rec.Body.String() != "<p>hi from js, mw</p>" {
		t.Fatalf("GET /hello: %d %q", rec.Code, rec.Body.String())
	}
	if rec := do("GET", "/users/ben"); rec.Code != 200 || rec.Body.String() != "user ben" {
		t.Fatalf("GET /users/ben: %d %q", rec.Code, rec.Body.String())
	}
	if rec := do("GET", "/silent"); rec.Code != 500 { // dev mode shows "returned without sending a response"
		t.Fatalf("GET /silent: %d %.200q", rec.Code, rec.Body.String())
	}
	// errors are pages, not JSON
	if rec := do("GET", "/boom"); rec.Code != 500 || strings.Contains(rec.Header().Get("Content-Type"), "json") {
		t.Fatalf("GET /boom: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := do("GET", "/shop/about"); rec.Code != 200 || rec.Body.String() != "about 1" {
		t.Fatalf("GET /shop/about: %d %q", rec.Code, rec.Body.String())
	}
	if rec := do("GET", "/shop"); rec.Code != 404 {
		t.Fatalf("GET /shop: %d, want 404", rec.Code)
	}
	if rec := do("GET", "/feed"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"api":true`) {
		t.Fatalf("GET /feed: %d %q", rec.Code, rec.Body.String())
	}
}
