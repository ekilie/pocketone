package main

// Typed API pages: each member of an extension's JS API as its own entry (signature, parameters, what it returns,
// examples), shown beside its Go counterpart. The JS side is parsed from the extension's TypeScript declarations
// (doc comments with @param, @returns, @example, and @go naming the Go function, method or type it matches); the Go
// side is read with go/doc from the contract packages the extension provides, examples (Example* functions in
// _test.go files) included.

import (
	"fmt"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------------------------------------------
// JSDoc

type jsDocParam struct{ Name, Desc string }

type jsDocInfo struct {
	Summary  string // the first paragraph
	Body     string // every paragraph, Markdown (code fences kept)
	Params   []jsDocParam
	Returns  string
	Examples []string // code, without fences
	Go       []string // @go targets: "kvv1.Get", "kvv1.Store.Get", "kvv1.Entry"
}

var jsTagRe = regexp.MustCompile(`^@(\w+)\s*(.*)$`)

// parseJSDoc reads a doc comment's lines (as collected, /** and */ included).
func parseJSDoc(lines []string) jsDocInfo {
	var d jsDocInfo
	var body []string
	tag, tagArg := "", ""
	var tagLines []string
	flush := func() {
		text := strings.TrimSpace(strings.Join(tagLines, "\n"))
		switch tag {
		case "param":
			name, desc, _ := strings.Cut(strings.TrimSpace(tagArg+" "+text), " ")
			name = strings.Trim(strings.TrimPrefix(name, "{"), "[]")
			if i := strings.Index(name, "}"); i >= 0 { // @param {type} name
				name = strings.TrimSpace(name[i+1:])
			}
			d.Params = append(d.Params, jsDocParam{Name: name, Desc: strings.TrimPrefix(strings.TrimSpace(desc), "- ")})
		case "returns", "return":
			d.Returns = strings.TrimSpace(tagArg + " " + text)
		case "example":
			code := strings.TrimSpace(tagArg + "\n" + text)
			code = regexp.MustCompile("(?s)^```\\w*\\n?(.*?)\\n?```$").ReplaceAllString(code, "$1")
			if code != "" {
				d.Examples = append(d.Examples, code)
			}
		case "go":
			for _, f := range strings.Fields(tagArg + " " + text) {
				d.Go = append(d.Go, f)
			}
		}
		tag, tagArg, tagLines = "", "", nil
	}
	for _, l := range lines {
		l = strings.TrimSpace(l)
		l = strings.TrimPrefix(l, "/**")
		l = strings.TrimSuffix(l, "*/")
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "* ") || l == "*" {
			l = strings.TrimPrefix(strings.TrimPrefix(l, "*"), " ")
		}
		if m := jsTagRe.FindStringSubmatch(l); m != nil && !strings.HasPrefix(l, "@link") {
			flush()
			tag, tagArg = m[1], m[2]
			continue
		}
		if tag != "" {
			tagLines = append(tagLines, l)
		} else {
			body = append(body, l)
		}
	}
	flush()
	d.Body = jsLinkRe.ReplaceAllString(strings.TrimSpace(strings.Join(body, "\n")), "`$1`")
	d.Summary = jsDocSummary(lines)
	return d
}

// ---------------------------------------------------------------------------------------------------------------
// TypeScript signatures

type tsParam struct {
	Name, Type string
	Optional   bool
}

type tsSig struct {
	Name     string
	Method   bool
	Params   []tsParam
	Returns  string // a method's return type; a property's type
	Readonly bool
}

// splitTop splits s at sep where no bracket is open.
func splitTop(s string, sep rune) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(', '[', '{', '<':
			depth++
		case ')', ']', '}', '>':
			if !(r == '>' && i > 0 && s[i-1] == '=') { // "=>" isn't a bracket
				depth--
			}
		default:
			if r == sep && depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// closing is the index of the bracket closing the one s[open] opens, or -1.
func closing(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{', '<':
			depth++
		case ')', ']', '}', '>':
			if s[i] == '>' && i > 0 && s[i-1] == '=' {
				continue
			}
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

var tsSpaceRe = regexp.MustCompile(`\s+`)

// parseTSMember reads one member's text ("get(name: string): any", "list(prefix?: string): KvEntry[]",
// "readonly id: string"), joined onto one line.
func parseTSMember(text string) (tsSig, bool) {
	t := strings.TrimSuffix(strings.TrimSpace(tsSpaceRe.ReplaceAllString(text, " ")), ";")
	t = strings.TrimSuffix(t, ",")
	var s tsSig
	if r, ok := strings.CutPrefix(t, "readonly "); ok {
		s.Readonly, t = true, r
	}
	t = jsKeywordRe.ReplaceAllString(t, "")
	m := regexp.MustCompile(`^([A-Za-z_$][\w$]*)(\??)`).FindStringSubmatch(t)
	if m == nil {
		return s, false
	}
	s.Name = m[1]
	rest := strings.TrimSpace(t[len(m[0]):])
	optional := m[2] == "?"
	if strings.HasPrefix(rest, "<") { // type parameters
		if c := closing(rest, 0); c > 0 {
			rest = strings.TrimSpace(rest[c+1:])
		}
	}
	if strings.HasPrefix(rest, "(") {
		c := closing(rest, 0)
		if c < 0 {
			return s, false
		}
		s.Method = true
		if inner := strings.TrimSpace(rest[1:c]); inner != "" {
			for _, p := range splitTop(inner, ',') {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				name, typ, _ := strings.Cut(p, ":")
				name = strings.TrimSpace(name)
				tp := tsParam{Name: strings.TrimSuffix(name, "?"), Type: strings.TrimSpace(typ), Optional: strings.HasSuffix(name, "?")}
				if strings.HasPrefix(tp.Name, "...") {
					tp.Optional = true
				}
				s.Params = append(s.Params, tp)
			}
		}
		s.Returns = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rest[c+1:]), ":"))
		return s, true
	}
	s.Returns = strings.TrimSpace(strings.TrimPrefix(rest, ":"))
	if optional {
		s.Returns += " (optional)"
	}
	return s, true
}

// display is the signature as the page shows it: "$kv.get(name: string): any".
func (s tsSig) display(prefix string) string {
	if !s.Method {
		return prefix + s.Name + ": " + s.Returns
	}
	var ps []string
	for _, p := range s.Params {
		opt := ""
		if p.Optional && !strings.HasPrefix(p.Name, "...") {
			opt = "?"
		}
		ps = append(ps, p.Name+opt+": "+p.Type)
	}
	out := prefix + s.Name + "(" + strings.Join(ps, ", ") + ")"
	if s.Returns != "" {
		out += ": " + s.Returns
	}
	return out
}

// ---------------------------------------------------------------------------------------------------------------
// The JS API as entries

type apiEntry struct {
	Path  string // "$kv.get", "$llm.text"
	Sig   tsSig
	Doc   jsDocInfo
	Group bool // a namespace of members ($llm.text), not a member itself
}

type apiType struct {
	Name   string
	Doc    jsDocInfo
	Fields []apiEntry // an object type's fields, when it has them
	Text   string     // the type as declared, when it isn't an object type
}

// memberText is a member's whole text, from line i through the line that balances its brackets.
func memberText(lines []string, i int) (string, int) {
	text := strings.TrimSpace(lines[i])
	depth := braceDepth(lines[i]) + strings.Count(stripJSStrings(lines[i]), "(") - strings.Count(stripJSStrings(lines[i]), ")")
	j := i
	for depth > 0 && j+1 < len(lines) {
		j++
		text += " " + strings.TrimSpace(lines[j])
		depth += braceDepth(lines[j]) + strings.Count(stripJSStrings(lines[j]), "(") - strings.Count(stripJSStrings(lines[j]), ")")
	}
	return text, j
}

// apiEntries lists an object type's (or namespace's) members, with groups for object-valued members and nested
// namespaces.
func apiEntries(body []string, prefix string) []apiEntry {
	var out []apiEntry
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
		if ns := jsInnerNSRe.FindStringSubmatch(t); ns != nil {
			inner, end := jsBlock(body, i)
			out = append(out, apiEntry{Path: prefix + ns[1], Doc: parseJSDoc(doc), Group: true})
			out = append(out, apiEntries(inner, prefix+ns[1]+".")...)
			i, doc = end, nil
			continue
		}
		stripped := jsKeywordRe.ReplaceAllString(t, "")
		if regexp.MustCompile(`^(readonly\s+)?[\w$]+\??\s*:\s*\{$`).MatchString(stripped) {
			name := regexp.MustCompile(`^(?:readonly\s+)?([\w$]+)`).FindStringSubmatch(stripped)[1]
			inner, end := jsBlock(body, i)
			out = append(out, apiEntry{Path: prefix + name, Doc: parseJSDoc(doc), Group: true})
			out = append(out, apiEntries(inner, prefix+name+".")...)
			i, doc = end, nil
			continue
		}
		text, end := memberText(body, i)
		if sig, ok := parseTSMember(text); ok && jsMemberRe.MatchString(stripped) {
			out = append(out, apiEntry{Path: prefix + sig.Name, Sig: sig, Doc: parseJSDoc(doc)})
		}
		i, doc = end, nil
	}
	return out
}

var jsInterfaceRe = regexp.MustCompile(`^(?:declare\s+)?(?:export\s+)?interface\s+([A-Za-z_$][\w$]*)(?:<[^{]*>)?(?:\s+extends\s+[^{]+)?\s*\{$`)

// nsTypes reads a namespace of types: its interfaces and type aliases, and those of the namespaces inside it, each
// named with the namespace ("pages.Context").
func nsTypes(body []string, prefix string) []apiType {
	var out []apiType
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
		d := parseJSDoc(doc)
		doc = nil
		switch {
		case jsInnerNSRe.MatchString(t):
			name := jsInnerNSRe.FindStringSubmatch(t)[1]
			inner, end := jsBlock(body, i)
			out = append(out, nsTypes(inner, prefix+name+".")...)
			i = end
		case jsInterfaceRe.MatchString(t):
			name := jsInterfaceRe.FindStringSubmatch(t)[1]
			inner, end := jsBlock(body, i)
			out = append(out, apiType{Name: prefix + name, Doc: d, Fields: apiEntries(inner, "")})
			i = end
		case jsAliasRe.MatchString(strings.TrimPrefix(t, "export ")):
			m := jsAliasRe.FindStringSubmatch(strings.TrimPrefix(t, "export "))
			at := apiType{Name: prefix + m[1], Doc: d}
			if strings.HasSuffix(strings.TrimSpace(m[2]), "{") {
				inner, end := jsBlock(body, i)
				at.Fields = apiEntries(inner, "")
				i = end
			} else {
				text, end := memberText(body, i)
				at.Text = strings.TrimSpace(text[strings.Index(text, "=")+1:])
				i = end
			}
			out = append(out, at)
		default:
			if braceDepth(body[i]) > 0 {
				_, i = jsBlock(body, i)
			}
		}
	}
	return out
}

// jsAPI is one extension's JS API: each global's entries, and the named types it declares.
type jsAPI struct {
	Globals    []jsGlobal
	Entries    map[string][]apiEntry
	GlobalDocs map[string]jsDocInfo
	Types      []apiType
}

func parseJSAPI(ts string) jsAPI {
	api := jsAPI{Globals: jsGlobals(ts), Entries: map[string][]apiEntry{}, GlobalDocs: map[string]jsDocInfo{}}
	lines := strings.Split(ts, "\n")
	aliases := map[string][]string{}
	var doc []string
	inDoc := false
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		t := strings.TrimSpace(raw)
		top := !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t")
		switch {
		case inDoc:
			doc = append(doc, t)
			if strings.Contains(t, "*/") {
				inDoc = false
			}
			continue
		case strings.HasPrefix(t, "/**") && top:
			doc = []string{t}
			inDoc = !strings.Contains(t, "*/")
			continue
		case t == "" || strings.HasPrefix(t, "//") || !top:
			continue
		}
		d := parseJSDoc(doc)
		doc = nil
		if m := jsAliasRe.FindStringSubmatch(t); m != nil {
			at := apiType{Name: m[1], Doc: d}
			if strings.HasSuffix(strings.TrimSpace(m[2]), "{") {
				body, end := jsBlock(lines, i)
				aliases[m[1]] = body
				at.Fields = apiEntries(body, "")
				i = end
			} else {
				text, end := memberText(lines, i)
				at.Text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text[strings.Index(text, "=")+1:]), " "))
				i = end
			}
			api.Types = append(api.Types, at)
			continue
		}
		if ns := jsNSRe.FindStringSubmatch(t); ns != nil {
			body, end := jsBlock(lines, i)
			api.GlobalDocs[ns[1]] = d
			if strings.HasPrefix(ns[1], "$") {
				api.Entries[ns[1]] = append(api.Entries[ns[1]], apiEntries(body, ns[1]+".")...)
			} else {
				api.Types = append(api.Types, nsTypes(body, ns[1]+".")...) // a namespace of types (pages, contracts)
			}
			i = end
			continue
		}
		if m := jsInterfaceRe.FindStringSubmatch(t); m != nil {
			body, end := jsBlock(lines, i)
			api.Types = append(api.Types, apiType{Name: m[1], Doc: d, Fields: apiEntries(body, "")})
			i = end
			continue
		}
		if m := jsVarRe.FindStringSubmatch(t); m != nil {
			name, expr := m[1], strings.TrimSpace(m[2])
			api.GlobalDocs[name] = d
			for _, id := range jsIdentRe.FindAllString(strings.TrimSuffix(expr, "{"), -1) {
				if body, ok := aliases[id]; ok {
					api.Entries[name] = append(api.Entries[name], apiEntries(body, name+".")...)
				}
			}
			if strings.HasSuffix(expr, "{") {
				body, end := jsBlock(lines, i)
				api.Entries[name] = append(api.Entries[name], apiEntries(body, name+".")...)
				i = end
			}
			continue
		}
		if m := jsDeclRe.FindStringSubmatch(t); m != nil {
			api.GlobalDocs[m[2]] = d
			if m[1] == "function" {
				text, end := memberText(lines, i)
				if sig, ok := parseTSMember(strings.TrimPrefix(text, "declare ")); ok {
					api.Entries[m[2]] = append(api.Entries[m[2]], apiEntry{Path: sig.Name, Sig: sig, Doc: d})
				}
				i = end
			}
		}
	}
	// a type alias used only to build a global ($llm's LlmAPI) is the global's, not a type of its own
	used := map[string]bool{}
	for _, g := range api.Globals {
		used[g.Name] = true
	}
	var types []apiType
	for _, at := range api.Types {
		isAPI := false
		for _, line := range lines {
			if m := jsVarRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil && regexp.MustCompile(`\b`+regexp.QuoteMeta(at.Name)+`\b`).MatchString(m[2]) {
				isAPI = true
			}
		}
		if !isAPI {
			types = append(types, at)
		}
	}
	api.Types = types
	return api
}

// ---------------------------------------------------------------------------------------------------------------
// The page

func typedPage(m *manifest, r jsRef, gopkgs []goRefPackage) string {
	title := jsTitle(m, r.Extension)
	api := parseJSAPI(r.Types)
	var b strings.Builder
	b.WriteString(refFrontMatter(title+" JavaScript API", "Every function and type "+title+" adds to pb_hooks and page code: signatures, parameters, what each returns, and examples. Generated from its TypeScript declarations."))
	fmt.Fprintf(&b, "# %s JavaScript API\n\n", title)
	if r.Extension == "contracts" {
		b.WriteString("Every contract an app's JS may touch: a hook for each of its operations (`on<Contract><Operation>`, which " +
			"may change the call's input or refuse it), `onContracts` to provide contracts from `pb_hooks`, and the contracts' types. " +
			"See [Contracts](/docs/contracts); their Go side is in the [Go API](/docs/reference/go).\n\n")
	} else {
		fmt.Fprintf(&b, "Everything the [%s](/docs/extensions/%s) extension adds to `pb_hooks` and page code, generated from the "+
			"TypeScript declarations it ships (your app's `types.d.ts` has the same, so your editor knows them). For how to use it, "+
			"see its docs; for Go, the [Go API](/docs/reference/go).\n\n", title, r.Extension)
	}
	var funcs []apiEntry
	for _, g := range api.Globals {
		entries := api.Entries[g.Name]
		if g.Kind == "function" {
			funcs = append(funcs, entries...)
			continue
		}
		if len(entries) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", g.Name)
		if d := api.GlobalDocs[g.Name]; d.Body != "" {
			b.WriteString(d.Body + "\n\n")
		}
		for _, e := range entries {
			writeEntry(&b, e, gopkgs)
		}
	}
	if len(funcs) > 0 {
		b.WriteString("## Functions\n\n")
		for _, e := range funcs {
			writeEntry(&b, e, gopkgs)
		}
	}
	if len(api.Types) > 0 {
		b.WriteString("## Types\n\n")
		for _, t := range api.Types {
			writeType(&b, t, gopkgs)
		}
	}
	return escapeEJS(b.String())
}

// goLinks is "Go: [`kvv1.Get`](...)" for an entry's @go targets that the Go reference has, "" when none.
func goLinks(targets []string, gopkgs []goRefPackage) string {
	var links []string
	for _, t := range targets {
		if u := goRefLink(gopkgs, t); u != "" {
			links = append(links, fmt.Sprintf("[`%s`](%s)", t, u))
		}
	}
	if len(links) == 0 {
		return ""
	}
	return "Go: " + strings.Join(links, ", ") + ".\n\n"
}

func writeEntry(b *strings.Builder, e apiEntry, gopkgs []goRefPackage) {
	if e.Group {
		fmt.Fprintf(b, "### %s\n\n", e.Path)
		if e.Doc.Body != "" {
			b.WriteString(e.Doc.Body + "\n\n")
		}
		return
	}
	heading := e.Path
	if e.Sig.Method {
		heading += "()"
	}
	fmt.Fprintf(b, "### %s\n\n", heading)
	prefix := strings.TrimSuffix(e.Path, e.Sig.Name)
	fmt.Fprintf(b, "```ts\n%s\n```\n\n", e.Sig.display(prefix))
	if e.Doc.Body != "" {
		b.WriteString(e.Doc.Body + "\n\n")
	}
	if e.Sig.Method && len(e.Sig.Params) > 0 {
		desc := map[string]string{}
		for _, p := range e.Doc.Params {
			desc[p.Name] = p.Desc
		}
		b.WriteString("| Parameter | Type | |\n|---|---|---|\n")
		for _, p := range e.Sig.Params {
			opt := ""
			if p.Optional {
				opt = " (optional)"
			}
			fmt.Fprintf(b, "| `%s` | `%s`%s | %s |\n", p.Name, strings.ReplaceAll(p.Type, "|", "\\|"), opt, cell(desc[p.Name]))
		}
		b.WriteString("\n")
	}
	if e.Sig.Method && e.Sig.Returns != "" && e.Sig.Returns != "void" {
		fmt.Fprintf(b, "Returns `%s`", strings.ReplaceAll(e.Sig.Returns, "`", "'"))
		if e.Doc.Returns != "" {
			b.WriteString(": " + e.Doc.Returns)
		}
		b.WriteString(".\n\n")
	}
	for _, ex := range e.Doc.Examples {
		fmt.Fprintf(b, "```js\n%s\n```\n\n", ex)
	}
	b.WriteString(goLinks(e.Doc.Go, gopkgs))
}

func writeType(b *strings.Builder, t apiType, gopkgs []goRefPackage) {
	fmt.Fprintf(b, "### %s\n\n", t.Name)
	if t.Doc.Body != "" {
		b.WriteString(t.Doc.Body + "\n\n")
	}
	if len(t.Fields) > 0 {
		b.WriteString("| Member | Type | |\n|---|---|---|\n")
		for _, f := range t.Fields {
			if f.Group {
				continue
			}
			typ := strings.TrimSuffix(f.Sig.Returns, " (optional)")
			name := f.Sig.Name
			if f.Sig.Method {
				typ = strings.TrimPrefix(f.Sig.display(""), f.Sig.Name)
				name += "()"
			} else if strings.HasSuffix(f.Sig.Returns, " (optional)") {
				name += "?"
			}
			if f.Sig.Readonly {
				name = "readonly " + name
			}
			fmt.Fprintf(b, "| `%s` | `%s` | %s |\n", name, strings.ReplaceAll(typ, "|", "\\|"), cell(f.Doc.Summary))
		}
		b.WriteString("\n")
	} else if t.Text != "" {
		fmt.Fprintf(b, "```ts\ntype %s = %s\n```\n\n", t.Name, t.Text)
	}
	b.WriteString(goLinks(t.Doc.Go, gopkgs))
}
