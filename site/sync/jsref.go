package main

// The JavaScript API reference (/docs/reference/javascript and a page per extension under it), from the TypeScript
// declarations `pocketone reference` lists (clicmdv1.Reference.JS): what each extension adds to pb_hooks and page code,
// exactly as it goes into an app's types.d.ts. Generated, so it can't drift from the code.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type jsRef struct {
	Extension string `json:"extension"`
	Types     string `json:"types"`
}

// jsGlobal is one top-level declaration a reader calls: a global variable, function, class or namespace.
type jsGlobal struct {
	Kind, Name, Doc string
}

var jsDeclRe = regexp.MustCompile(`^declare\s+(var|let|const|function|class|namespace|module|enum)\s+([A-Za-z_$][\w$.]*)`)

// jsGlobals lists the top-level declarations in ts, each with its doc comment's first paragraph. Function overloads
// are listed once.
func jsGlobals(ts string) []jsGlobal {
	var out []jsGlobal
	seen := map[string]bool{}
	depth := 0
	var doc []string
	inDoc := false
	for _, line := range strings.Split(ts, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case inDoc:
			doc = append(doc, t)
			if strings.Contains(t, "*/") {
				inDoc = false
			}
			continue
		case strings.HasPrefix(t, "/**") && depth == 0:
			doc = []string{t}
			inDoc = !strings.Contains(t, "*/")
			continue
		case strings.HasPrefix(t, "//") || t == "":
			continue
		}
		if depth == 0 {
			if m := jsDeclRe.FindStringSubmatch(t); m != nil && !seen[m[2]] {
				seen[m[2]] = true
				out = append(out, jsGlobal{Kind: m[1], Name: m[2], Doc: jsDocSummary(doc)})
			}
			doc = nil
		}
		depth += strings.Count(stripJSStrings(t), "{") - strings.Count(stripJSStrings(t), "}")
		if depth < 0 {
			depth = 0
		}
	}
	return out
}

var jsStringRe = regexp.MustCompile("\"(?:[^\"\\\\]|\\\\.)*\"|'(?:[^'\\\\]|\\\\.)*'|`[^`]*`")

// stripJSStrings drops string literals and a trailing comment, so braces in them don't count toward nesting.
func stripJSStrings(s string) string {
	s = jsStringRe.ReplaceAllString(s, `""`)
	if i := strings.Index(s, "//"); i >= 0 {
		s = s[:i]
	}
	return s
}

// jsDocSummary is a doc comment's first paragraph as one line, without its tags or code samples.
func jsDocSummary(lines []string) string {
	var words []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		l = strings.TrimPrefix(l, "/**")
		l = strings.TrimSuffix(l, "*/")
		l = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "*"))
		if strings.HasPrefix(l, "@") || strings.HasPrefix(l, "```") {
			break
		}
		if l == "" {
			if len(words) > 0 {
				break
			}
			continue
		}
		words = append(words, l)
	}
	return jsLinkRe.ReplaceAllString(strings.Join(words, " "), "`$1`")
}

// jsLinkRe is a JSDoc {@link Name} (or {@link Name text}), shown as code.
var jsLinkRe = regexp.MustCompile(`\{@link\s+([^\s}]+)[^}]*\}`)

// jsMember is one member of a global's object type: "text.completeAsync(opts: LlmTextInput)" and its doc.
type jsMember struct {
	Sig, Doc string
	Group    bool // an object-valued member or nested namespace ($llm.text): a heading over its own members
}

var (
	jsAliasRe   = regexp.MustCompile(`^(?:declare\s+)?type\s+([A-Za-z_$][\w$]*)\s*(?:<[^>]*>)?\s*=\s*(.*)$`)
	jsVarRe     = regexp.MustCompile(`^declare\s+(?:var|let|const)\s+([A-Za-z_$][\w$]*)\s*:\s*(.*)$`)
	jsMemberRe  = regexp.MustCompile(`^(readonly\s+)?([A-Za-z_$][\w$]*)(\?)?\s*([:(<])`)
	jsIdentRe   = regexp.MustCompile(`[A-Za-z_$][\w$]*`)
	jsNSRe      = regexp.MustCompile(`^declare\s+namespace\s+([A-Za-z_$][\w$.]*)\s*\{$`)
	jsInnerNSRe = regexp.MustCompile(`^(?:export\s+)?namespace\s+([A-Za-z_$][\w$]*)\s*\{$`)
	jsKeywordRe = regexp.MustCompile(`^(export\s+)?(function|const|let|var)\s+`)
)

// braceDepth is how much a line opens: { minus }, outside strings and comments.
func braceDepth(line string) int {
	t := stripJSStrings(strings.TrimSpace(line))
	return strings.Count(t, "{") - strings.Count(t, "}")
}

// jsBlock returns the lines inside the object type that opens on lines[start] (which ends with "{"), and the index
// of the line that closes it.
func jsBlock(lines []string, start int) (body []string, end int) {
	depth := braceDepth(lines[start])
	for i := start + 1; i < len(lines); i++ {
		d := braceDepth(lines[i])
		if depth+d <= 0 {
			return lines[start+1 : i], i
		}
		depth += d
	}
	return lines[start+1:], len(lines) - 1
}

// jsMembers lists the members of the object type in body (one level, and into object-valued members), prefixed.
func jsMembers(body []string, prefix string) []jsMember {
	var out []jsMember
	var doc []string
	inDoc := false
	for i := 0; i < len(body); i++ {
		t := strings.TrimSpace(body[i])
		switch {
		case inDoc:
			doc = append(doc, t)
			if strings.Contains(t, "*/") {
				inDoc = false
			}
			continue
		case strings.HasPrefix(t, "/**"):
			doc = []string{t}
			inDoc = !strings.Contains(t, "*/")
			continue
		case t == "" || strings.HasPrefix(t, "//"):
			continue
		}
		// a nested namespace ($wasm.run): its members under its name, like an object-valued member
		if ns := jsInnerNSRe.FindStringSubmatch(t); ns != nil {
			inner, end := jsBlock(body, i)
			out = append(out, jsMember{Sig: prefix + ns[1], Doc: jsDocSummary(doc), Group: true})
			out = append(out, jsMembers(inner, prefix+ns[1]+".")...)
			i, doc = end, nil
			continue
		}
		// a namespace's members: "function run(...)", "const version: string", maybe exported
		t = jsKeywordRe.ReplaceAllString(t, "")
		m := jsMemberRe.FindStringSubmatch(t)
		opens := braceDepth(body[i]) > 0
		if m == nil {
			if opens {
				_, i = jsBlock(body, i)
			}
			doc = nil
			continue
		}
		name := prefix + m[2]
		switch {
		case m[4] == ":" && strings.HasSuffix(t, "{") && regexp.MustCompile(`^(readonly\s+)?[\w$]+\??\s*:\s*\{$`).MatchString(t):
			// an object-valued member: its own members, under its name
			inner, end := jsBlock(body, i)
			out = append(out, jsMember{Sig: name, Doc: jsDocSummary(doc), Group: true})
			out = append(out, jsMembers(inner, name+".")...)
			i = end
		case m[4] == "(" || m[4] == "<":
			sig := name + "(…)"
			if open := strings.Index(t, "("); open >= 0 {
				if params, ok := balanced(t[open:]); ok {
					sig = name + params
				}
			}
			out = append(out, jsMember{Sig: sig, Doc: jsDocSummary(doc)})
			if opens {
				_, i = jsBlock(body, i)
			}
		default:
			typ := strings.TrimSpace(strings.TrimPrefix(t[len(m[0])-1:], ":"))
			sig := name
			if !opens && typ != "" && len(typ) <= 60 {
				sig += ": " + strings.TrimSuffix(typ, ";")
			}
			out = append(out, jsMember{Sig: sig, Doc: jsDocSummary(doc)})
			if opens {
				_, i = jsBlock(body, i)
			}
		}
		doc = nil
	}
	return out
}

// balanced is the parenthesised group s starts with ("(a: string, b?: number)"), when it closes on this line.
func balanced(s string) (string, bool) {
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[:i+1], true
			}
		}
	}
	return "", false
}

// jsGlobalMembers is, for each global variable declared with an object type (inline, or a type alias declared in the
// same file, or both joined with &), its members.
func jsGlobalMembers(ts string) map[string][]jsMember {
	lines := strings.Split(ts, "\n")
	aliases := map[string][]string{}
	for i := 0; i < len(lines); i++ {
		if m := jsAliasRe.FindStringSubmatch(strings.TrimSpace(lines[i])); m != nil && strings.HasSuffix(strings.TrimSpace(m[2]), "{") {
			body, end := jsBlock(lines, i)
			aliases[m[1]] = body
			i = end
		}
	}
	out := map[string][]jsMember{}
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], " ") || strings.HasPrefix(lines[i], "\t") {
			continue
		}
		if ns := jsNSRe.FindStringSubmatch(strings.TrimSpace(lines[i])); ns != nil {
			// a namespace used as a global ($wasm): its functions and constants
			body, end := jsBlock(lines, i)
			if ms := jsMembers(body, ns[1]+"."); len(ms) > 0 {
				out[ns[1]] = append(out[ns[1]], ms...)
			}
			i = end
			continue
		}
		m := jsVarRe.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if m == nil {
			continue
		}
		name, expr := m[1], strings.TrimSpace(m[2])
		var ms []jsMember
		for _, id := range jsIdentRe.FindAllString(strings.TrimSuffix(expr, "{"), -1) {
			if body, ok := aliases[id]; ok {
				ms = append(ms, jsMembers(body, name+".")...)
			}
		}
		if strings.HasSuffix(expr, "{") {
			body, end := jsBlock(lines, i)
			ms = append(ms, jsMembers(body, name+".")...)
			i = end
		}
		if len(ms) > 0 {
			out[name] = ms
		}
	}
	return out
}

func jsPageURL(ext string) string { return "/docs/reference/javascript/" + ext }

// jsTitle is the extension's name as the docs show it.
func jsTitle(m *manifest, ext string) string {
	if ext == "contracts" {
		return "Contract hooks"
	}
	for _, p := range m.Extensions {
		if p.Name == ext && p.Title != "" {
			return p.Title
		}
	}
	return ext
}

// jsRefs is the reference's entries, sorted by the title the docs show.
func jsRefs(m *manifest) []jsRef {
	if m.ref == nil {
		return nil
	}
	refs := append([]jsRef(nil), m.ref.JS...)
	sort.SliceStable(refs, func(i, j int) bool {
		// the contracts' hooks go last: they're the general part, the extensions' globals the specific one
		if (refs[i].Extension == "contracts") != (refs[j].Extension == "contracts") {
			return refs[j].Extension == "contracts"
		}
		return jsTitle(m, refs[i].Extension) < jsTitle(m, refs[j].Extension)
	})
	return refs
}

// jsIndex is /docs/reference/javascript: every extension that adds to the JS API, with its globals.
func jsIndex(m *manifest, pbNS []string, pbpkgs []goRefPackage) string {
	var b strings.Builder
	b.WriteString(refFrontMatter("JavaScript API", "The whole JavaScript API of pb_hooks and page code: what core gives every app ($app, the hooks, the classes and every type) and what each extension adds, generated from the TypeScript declarations."))
	b.WriteString("# JavaScript API\n\n")
	b.WriteString("Everything `pb_hooks` and page code can call, generated from the TypeScript declarations PocketOne ships " +
		"(the same ones your app's `types.d.ts` gets, so your editor knows them too).\n\n")
	if len(pbNS) > 0 {
		b.WriteString("## PocketBase\n\n")
		b.WriteString("What core gives every app: [the globals, hooks, functions and classes](" + pbJSURL + ") (`$app`, `$apis`, " +
			"`$security`, `routerAdd`, `cronAdd`, `onRecordCreate`, `new Record()`...), and the types they hand you, one page per Go " +
			"namespace:\n\n")
		var own, libs []string
		for _, ns := range pbNS {
			l := fmt.Sprintf("[%s](%s)", ns, pbNSURL(ns))
			if pbGoPage(ns, pbpkgs) != "" || ns == "dbx" {
				own = append(own, l)
			} else {
				libs = append(libs, l)
			}
		}
		b.WriteString("- PocketBase's: " + strings.Join(own, ", ") + "\n")
		if len(libs) > 0 {
			b.WriteString("- From the Go libraries it builds on: " + strings.Join(libs, ", ") + "\n")
		}
		b.WriteString("\n## Extensions\n\n")
	}
	b.WriteString("Each PocketOne extension adds to that: a global such as `$llm`, functions to bind hooks, types. Each page below " +
		"lists what one extension adds. An extension that's off on your app adds nothing.\n\n")
	b.WriteString("| Extension | Adds |\n|---|---|\n")
	for _, r := range jsRefs(m) {
		var names []string
		for _, g := range jsGlobals(r.Types) {
			names = append(names, "`"+g.Name+"`")
		}
		if len(names) > 8 {
			names = append(names[:8], fmt.Sprintf("and %d more", len(names)-8))
		}
		adds := strings.Join(names, ", ")
		if adds == "" {
			adds = "types"
		}
		fmt.Fprintf(&b, "| [%s](%s) | %s |\n", jsTitle(m, r.Extension), jsPageURL(r.Extension), adds)
	}
	b.WriteString("\n")
	return b.String()
}

// jsPage is one extension's page: its globals, then its declarations in full.
func jsPage(m *manifest, r jsRef) string {
	title := jsTitle(m, r.Extension)
	var b strings.Builder
	b.WriteString(refFrontMatter(title+" JavaScript API", "What "+title+" adds to the JavaScript API of pb_hooks and page code, generated from its TypeScript declarations."))
	fmt.Fprintf(&b, "# %s JavaScript API\n\n", title)
	if r.Extension == "contracts" {
		b.WriteString("Every contract an app's JS may touch: a hook for each of its operations (`on<Contract><Operation>`, which " +
			"may change the call's input or refuse it), `onContracts` to provide contracts from `pb_hooks`, and the contracts' types. " +
			"See [Contracts](/docs/contracts).\n\n")
	} else {
		fmt.Fprintf(&b, "What the [%s](/docs/extensions/%s) extension adds to `pb_hooks` and page code, generated from the TypeScript "+
			"declarations it ships (your app's `types.d.ts` has the same). For how to use it, see its docs.\n\n", title, r.Extension)
	}
	members := jsGlobalMembers(r.Types)
	gs := jsGlobals(r.Types)
	if len(gs) > 1 {
		b.WriteString("## Globals\n\n| | |\n|---|---|\n")
		for _, g := range gs {
			name := "`" + g.Name + "`"
			if len(members[g.Name]) > 0 {
				name = "[`" + g.Name + "`](#" + anchor(g.Name) + ")"
			}
			fmt.Fprintf(&b, "| %s | %s |\n", name, cell(g.Doc))
		}
		b.WriteString("\n")
	}
	for _, g := range gs {
		ms := members[g.Name]
		if len(ms) == 0 {
			if len(gs) == 1 {
				fmt.Fprintf(&b, "## %s\n\n%s\n\n", g.Name, g.Doc)
			}
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", g.Name)
		if g.Doc != "" {
			b.WriteString(g.Doc + "\n\n")
		}
		writeMembers(&b, ms)
	}
	b.WriteString("## Declarations\n\nThe declarations in full, with every type and option.\n\n")
	b.WriteString("```ts\n" + escapeEJS(strings.TrimSpace(r.Types)) + "\n```\n")
	return b.String()
}

// writeMembers writes a global's members: the ones directly on it as a list, then each group ($llm.text) under its own
// heading with its members.
func writeMembers(b *strings.Builder, ms []jsMember) {
	var groups []jsMember
	for _, m := range ms {
		if m.Group {
			groups = append(groups, m)
		}
	}
	// a member belongs to the innermost group whose name prefixes it
	groupOf := func(m jsMember) string {
		best := ""
		for _, g := range groups {
			if strings.HasPrefix(m.Sig, g.Sig+".") && len(g.Sig) > len(best) {
				best = g.Sig
			}
		}
		return best
	}
	item := func(m jsMember) {
		if m.Doc != "" {
			fmt.Fprintf(b, "- `%s`: %s\n", m.Sig, m.Doc)
		} else {
			fmt.Fprintf(b, "- `%s`\n", m.Sig)
		}
	}
	any := false
	for _, m := range ms {
		if !m.Group && groupOf(m) == "" {
			item(m)
			any = true
		}
	}
	if any {
		b.WriteString("\n")
	}
	for _, g := range groups {
		var items []jsMember
		for _, m := range ms {
			if !m.Group && groupOf(m) == g.Sig {
				items = append(items, m)
			}
		}
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(b, "### %s\n\n", g.Sig)
		if g.Doc != "" {
			b.WriteString(g.Doc + "\n\n")
		}
		for _, m := range items {
			item(m)
		}
		b.WriteString("\n")
	}
}

// jsSurface is an extension page's pointer to its JavaScript API page, "" when it adds none.
func jsSurface(m *manifest, ext string) string {
	for _, r := range jsRefs(m) {
		if r.Extension == ext {
			return fmt.Sprintf("### JavaScript API\n\nWhat it adds to `pb_hooks` and page code: [%s JavaScript API](%s).\n\n", jsTitle(m, ext), jsPageURL(ext))
		}
	}
	return ""
}

// jsRefNav is the JavaScript API's own sidebar (toc-javascript): PocketBase's globals and namespaces, the Go libraries'
// namespaces, then each extension's page.
func jsRefNav(m *manifest, pbNS []string, pbpkgs []goRefPackage) string {
	var b strings.Builder
	b.WriteString("<!-- Generated by site/sync from the TypeScript declarations. -->\n\n# JavaScript API\n\n" + backLink + "- [Overview](/docs/reference/javascript)\n")
	if len(pbNS) > 0 {
		b.WriteString("\n# PocketBase\n\n- [Globals, hooks and classes](" + pbJSURL + ")\n")
		var libs []string
		for _, ns := range pbNS {
			if pbGoPage(ns, pbpkgs) != "" || ns == "dbx" {
				fmt.Fprintf(&b, "- [%s](%s)\n", ns, pbNSURL(ns))
				if ns == "core" {
					for _, f := range coreFamilies {
						fmt.Fprintf(&b, "- [core: %s](%s)\n", f.Title, pbNSURL(ns)+"/"+f.Slug)
					}
				}
			} else {
				libs = append(libs, ns)
			}
		}
		if len(libs) > 0 {
			b.WriteString("\n# Go libraries\n\n")
			for _, ns := range libs {
				fmt.Fprintf(&b, "- [%s](%s)\n", ns, pbNSURL(ns))
			}
		}
	}
	b.WriteString("\n# Extensions\n\n")
	for _, r := range jsRefs(m) {
		fmt.Fprintf(&b, "- [%s](%s)\n", jsTitle(m, r.Extension), jsPageURL(r.Extension))
	}
	return b.String()
}
