package pages

import (
	"strings"
	"testing"
)

func TestPluginStringsRequireFirst(t *testing.T) {
	greet := `module.exports = (o) => ({ name: 'greet', extendContext(ctx) { ctx.greet = 'hi ' + (o.who || 'pkg') } })`
	cases := []struct {
		name  string
		files map[string]string
		code  int
		want  string
	}{
		{"package from node_modules", map[string]string{
			"+config.js":                  "module.exports = { plugins: ['greet'] }",
			"node_modules/greet/index.js": greet,
			"index.ejs":                   "<%= ctx.greet %>",
		}, 200, "hi pkg"},
		{"pb_pages file as a string", map[string]string{
			"+config.js":        "module.exports = { plugins: ['./_private/greet.js'] }",
			"_private/greet.js": greet,
			"index.ejs":         "<%= ctx.greet %>",
		}, 200, "hi pkg"},
		{"default export", map[string]string{
			"+config.js":                  "module.exports = { plugins: ['greet'] }",
			"node_modules/greet/index.js": strings.Replace(greet, "module.exports =", "exports.default =", 1),
			"index.ejs":                   "<%= ctx.greet %>",
		}, 200, "hi pkg"},
		{"options with a required factory", map[string]string{
			"+config.js":                  "module.exports = { plugins: [{ factory: require('greet'), who: 'Ben' }] }",
			"node_modules/greet/index.js": greet,
			"index.ejs":                   "<%= ctx.greet %>",
		}, 200, "hi Ben"},
		{"a _private file by bare name (the datastar docs example)", map[string]string{
			"+config.js":        "module.exports = { plugins: ['realtime', 'greet'] }",
			"_private/greet.js": greet,
			"index.ejs":         "<%= ctx.greet %>|<%= typeof ctx.realtime.send %>",
		}, 200, "hi pkg|function"},
		{"_private wins over node_modules", map[string]string{
			"+config.js":                  "module.exports = { plugins: ['greet'] }",
			"_private/greet.js":           strings.Replace(greet, "'pkg'", "'private'", 1),
			"node_modules/greet/index.js": greet,
			"index.ejs":                   "<%= ctx.greet %>",
		}, 200, "hi private"},
		{"a _private file wins over the built-in", map[string]string{
			"+config.js":       "module.exports = { plugins: ['auth'] }",
			"_private/auth.js": `module.exports = () => ({ name: 'my-auth', extendContext(ctx) { ctx.mine = 'mine' } })`,
			"index.ejs":        "<%= ctx.mine %>|<%= ctx.session === undefined %>",
		}, 200, "mine|true"},
		{"no module by that name: built-in", map[string]string{
			"+config.js": "module.exports = { plugins: ['auth', 'realtime'] }",
			"index.ejs":  "<%= typeof ctx.session.signOut %>|<%= typeof ctx.realtime.send %>",
		}, 200, "function|function"},
		{"a package wins over the built-in", map[string]string{
			"+config.js":                     "module.exports = { plugins: ['realtime'] }",
			"node_modules/realtime/index.js": `module.exports = () => ({ name: 'my-realtime', extendContext(ctx) { ctx.mine = 'mine' } })`,
			"index.ejs":                      "<%= ctx.mine %>|<%= ctx.realtime === undefined %>",
		}, 200, "mine|true"},
	}
	for _, c := range cases {
		code, body := get(t, writeTree(t, c.files), "/", true)
		if code != c.code || body != c.want {
			t.Fatalf("%s: %d %.300q, want %q", c.name, code, body, c.want)
		}
	}

	fails := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"unknown name", map[string]string{
			"+config.js": "module.exports = { plugins: ['realtme'] }",
			"index.ejs":  "x",
		}, "realtme&#34;: require failed (built-in plugins: auth, realtime)"},
		{"a package that throws doesn't fall back to the built-in", map[string]string{
			"+config.js":                 "module.exports = { plugins: ['auth'] }",
			"node_modules/auth/index.js": "throw new Error('broken auth package')",
			"index.ejs":                  "x",
		}, "broken auth package"},
		{"a module that isn't a factory", map[string]string{
			"+config.js":                  "module.exports = { plugins: ['notes'] }",
			"node_modules/notes/index.js": "module.exports = 'just text'",
			"index.ejs":                   "x",
		}, "must be a built-in name, a module name or a plugin factory"},
	}
	for _, c := range fails {
		code, body := get(t, writeTree(t, c.files), "/", true)
		if code != 500 || !strings.Contains(body, c.want) {
			t.Fatalf("%s: %d %.400q, want it to mention %q", c.name, code, body, c.want)
		}
	}
}

func TestBareRequireOrder(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"_private/fmt.js":                "module.exports = 'root private'",
		"blog/_private/fmt.js":           "module.exports = 'blog private'",
		"node_modules/fmt/index.js":      "module.exports = 'package'",
		"node_modules/only-pkg/index.js": "module.exports = 'only package'",
		"index.ejs":                      "<%= require('fmt') %>|<%= require('only-pkg') %>",
		"blog/index.ejs":                 "<%= require('fmt') %>",
		"blog/_private/use.ejs":          "<%= require('fmt') %>",
		"blog/page.ejs":                  "<%- include('use') %>",
		"docs/index.ejs":                 "<%= require('fmt') %>",
	})
	for p, want := range map[string]string{
		"/":          "root private|only package", // _private first, node_modules when there's no _private file
		"/blog":      "blog private",              // the nearest _private folder wins
		"/blog/page": "blog private",              // from inside a _private folder, its own folder's _private
		"/docs":      "root private",              // up the chain to pb_pages/_private
	} {
		if code, body := get(t, dir, p, true); code != 200 || body != want {
			t.Errorf("%s: %d %.200q, want %q", p, code, body, want)
		}
	}
}
