package pages

import "testing"

func TestEJSLocals(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"_private/card.ejs": "<h2><%= title %></h2><% if (locals.subtitle) { %><p><%= locals.subtitle %></p><% } %>|<%= Object.keys(locals).join(',') %>",
		"index.ejs":         "<%- include('card', { title: 'A' }) %>/<%- include('card', { title: 'B', subtitle: 'b' }) %>/<%= typeof locals %>:<%= Object.keys(locals).length %>",
	})
	code, body := get(t, dir, "/", true)
	want := "<h2>A</h2>|title/<h2>B</h2><p>b</p>|title,subtitle/object:0"
	if code != 200 || body != want {
		t.Fatalf("%d %q, want %q", code, body, want)
	}
}
