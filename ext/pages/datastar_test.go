package pages

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDatastarHelpers(t *testing.T) {
	addon, _ := os.ReadFile("addons/datastar.js")
	dir := writeTree(t, map[string]string{
		"+config.js":           "module.exports = { plugins: ['datastar'] }",
		"_private/datastar.js": string(addon),
		"h/+api.js": `module.exports = { post(ctx) {
			const ds = ctx.datastar
			const t = ds.readSignals(ctx.req, { kept: 1 })
			const t2 = ds.readSignals({})
			ds.consoleLog('hi "there"')
			ds.consoleError(new Error('bad'))
			ds.redirect('/next')
			ds.dispatchCustomEvent('saved', { id: t.id, also: t2.id }, { selector: '#form' })
			ds.replaceURL('/x?y=1')
			ds.prefetch(['/a', '/b'])
		} }`,
		"index.ejs": "<%- ctx.datastar.scripts() %>",
	})
	h, _ := newServer(t, dir)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/h", strings.NewReader(`{"id":7}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		`data: elements <script data-effect="el.remove()">console.log("hi \"there\"")</script>`,
		`console.error("bad")`,
		`window.location.href = "/next"`,
		`document.querySelectorAll("#form").forEach(function (el) { el.dispatchEvent(new CustomEvent("saved", {"bubbles":true,"cancelable":true,"composed":true,"detail":{"id":7,"also":7}})) })`,
		`window.history.replaceState({}, "", "/x?y=1")`,
		`data: elements <script type="speculationrules">{"prefetch":[{"source":"list","urls":["/a","/b"]}]}</script>`,
		"data: selector body\ndata: mode append",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	if strings.Count(body, "event: datastar-patch-elements") != 6 {
		t.Errorf("want 6 patch events, got:\n%s", body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(rec.Body.String(), "starfederation/datastar@1.0.5/bundles/datastar.js") {
		t.Errorf("scripts() should load Datastar 1.0.5: %.300q", rec.Body.String())
	}
}
