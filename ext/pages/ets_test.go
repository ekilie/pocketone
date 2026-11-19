package pages

import (
	"strings"
	"testing"
)

// .ets templates are EJS whose tags hold TypeScript: types are stripped, the rest renders like .ejs.
func TestETSTemplates(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"+layout.ets": "<main><%- slot() %></main>",
		"index.ets": `<%
type Item = { name: string; price: number }
const items: Item[] = [{ name: 'a', price: 150 }, { name: 'b', price: 25 }]
const total = (xs: Item[]): number => xs.reduce((n, x) => n + x.price, 0)
%><% for (const item of items as Item[]) { %><%- include('row', { item }) %><% } %>|<%= total(items) %>`,
		"_private/row.ets":  `<% const it = item as { name: string } %><b><%= it.name %></b>`,
		"wait.ets":          `<% const v: number = await Promise.resolve(2) %><%= v * 2 %>`,
		"robots.txt.ets":    `<% const ok: boolean = true %>User-agent: <%= ok ? '*' : '' %>`,
		"broken.ets":        "line1\nline2\n<% const x: = 1 %>\n",
		"throws.ets":        "<% const n: number = 1 %>\nline2\n<%= (nope as any).x %>\n",
		"legacy.ejs":        `<%= 'still js' %>`,
		"products/[id].ets": `<%= (params.id as string).toUpperCase() %>`,
	})
	for path, want := range map[string]string{
		"/":            "<main><b>a</b><b>b</b>|175</main>",
		"/wait":        "<main>4</main>",
		"/robots.txt":  "User-agent: *", // not HTML, so no layout
		"/legacy":      "<main>still js</main>",
		"/products/x1": "<main>X1</main>",
	} {
		if code, body := get(t, dir, path, false); code != 200 || body != want {
			t.Errorf("%s: %d %q, want %q", path, code, body, want)
		}
	}
	code, body := get(t, dir, "/broken", true)
	if code != 500 || !strings.Contains(body, "broken.ets") || !strings.Contains(body, "line 3") {
		t.Errorf("/broken: %d %.300s", code, body)
	}
	// a runtime error in an .ets template names its template line, as in .ejs
	code, body = get(t, dir, "/throws", true)
	if code != 500 || !strings.Contains(body, "throws.ets:3") || !strings.Contains(body, "nope") {
		t.Errorf("/throws: %d %.300s", code, body)
	}
}

// A folder can't have a page as both .ets and .ejs.
func TestETSAndEJSSameStem(t *testing.T) {
	dir := writeTree(t, map[string]string{"+layout.ets": "a", "+layout.ejs": "b"})
	if _, errs := Build(dir); len(errs) == 0 {
		t.Fatal("want an error for +layout.ets beside +layout.ejs")
	}
}
