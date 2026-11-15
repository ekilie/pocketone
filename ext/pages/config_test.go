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
