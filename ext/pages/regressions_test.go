package pages

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A require cycle gets the partly built exports (as in Node and 0.x) instead of overflowing the Go stack.
func TestRequireCycle(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"_private/a.js": `exports.name = 'a'; const b = require('./b'); exports.fromB = b.seen`,
		"_private/b.js": `const a = require('./a'); exports.seen = 'b saw ' + a.name`,
		"index.ejs":     `<%= require('./_private/a.js').fromB %>`,
	})
	for i := 0; i < 2; i++ { // the second request uses the cached exports
		code, body := get(t, dir, "/", true)
		if code != 200 || body != "b saw a" {
			t.Fatalf("request %d: %d %.300q", i, code, body)
		}
	}
}

// A +config.js that exports a function (0.x's form) is an error, not silently ignored.
func TestConfigFunctionExport(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"+config.js": `module.exports = (api) => ({ plugins: ['auth'] })`,
		"index.ejs":  `ok`,
	})
	code, body := get(t, dir, "/", true)
	if code != 500 || !strings.Contains(body, "not a function") {
		t.Fatalf("%d %.300q", code, body)
	}
}

// After a build with errors, a later build that still has errors replaces it, so new pages are served.
func TestPartialBuildIsReplaced(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"index.ejs": `home`,
		"+load.js":  `module.exports = () => ({})`, // a 0.x file: a build error
	})
	h, p := newServer(t, dir)
	dev := true
	p.cfg.Dev = &dev
	fetch := func(path string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		b, _ := io.ReadAll(rec.Result().Body)
		return rec.Code, string(b)
	}
	if code, body := fetch("/"); code != 200 || body != "home" {
		t.Fatalf("/: %d %.200q", code, body)
	}
	if len(p.buildErrs) == 0 {
		t.Fatal("expected a build error for +load.js")
	}
	if err := os.WriteFile(filepath.Join(dir, "new.ejs"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	p.refresh(true)
	if code, body := fetch("/new"); code != 200 || body != "new" {
		t.Fatalf("/new after a second partial build: %d %.200q", code, body)
	}
}

// A bare require finds the app folder's node_modules (pb_pages' parent), where 0.x sites keep package.json, and a
// missing one names the folders searched.
func TestRequireAppNodeModules(t *testing.T) {
	root := writeTree(t, map[string]string{
		"node_modules/greet/package.json": `{"main":"lib/g.js"}`,
		"node_modules/greet/lib/g.js":     `module.exports = (n) => 'hi ' + n`,
		"pb_pages/index.ejs":              `<%= require('greet')('Ben') %>`,
		"pb_pages/missing.ejs":            `<%= require('nope-not-here') %>`,
	})
	dir := filepath.Join(root, "pb_pages")
	if code, body := get(t, dir, "/", true); code != 200 || body != "hi Ben" {
		t.Fatalf("/: %d %.300q", code, body)
	}
	code, body := get(t, dir, "/missing", true)
	if code != 500 || !strings.Contains(body, "looked in") || strings.Contains(body, "Invalid module") {
		t.Fatalf("/missing: %d %.400q", code, body)
	}
}
