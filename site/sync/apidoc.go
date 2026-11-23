package main

// Typed API pages: each member of an extension's JS API as its own entry (signature, parameters, what it returns,
// examples), shown beside its Go counterpart. The JS side is parsed from the extension's TypeScript declarations
// (doc comments with @param, @returns, @example, and @go naming the Go function, method or type it matches); the Go
// side is read with go/doc from the contract packages the extension provides, examples (Example* functions in
// _test.go files) included.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
			}
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
// Go, with go/doc

type goSym struct {
	Sig      string   // "func Get(app core.App, name string, dst any) (found bool, err error)"
	Doc      string   // the doc comment as Markdown (go/doc's printer: code blocks and links kept)
	Examples []string // each Example's code
	Fields   []goField
	Decl     string // a struct's declaration as written, comments included
	Comment  string // the doc comment as Go source: "// Get decodes ..." lines, to show above the signature
	Import   string // the package's import path
	Pkg      string // the package's name
}

type goField struct{ Name, Type, Doc string }

// goPackages reads the Go packages in dirs (relative to root), by package name: "kvv1" -> its symbols ("Get",
// "Store.Get", "Entry").
func goPackages(root string, dirs []string) map[string]map[string]goSym {
	out := map[string]map[string]goSym{}
	for _, dir := range dirs {
		fset := token.NewFileSet()
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			continue
		}
		var files []*ast.File
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			f, err := parser.ParseFile(fset, filepath.Join(root, dir, e.Name()), nil, parser.ParseComments)
			if err == nil {
				files = append(files, f)
			}
		}
		if len(files) == 0 {
			continue
		}
		p, err := doc.NewFromFiles(fset, files, "pocketone.dev/"+filepath.ToSlash(dir))
		if err != nil {
			continue
		}
		syms := map[string]goSym{}
		print := func(n any) string {
			var b bytes.Buffer
			_ = (&printer.Config{Mode: printer.UseSpaces, Tabwidth: 4}).Fprint(&b, fset, n)
			return b.String()
		}
		examples := func(exs []*doc.Example) []string {
			var out []string
			for _, ex := range exs {
				// the example's body, without its braces, its statements as written
				lines := strings.Split(print(ex.Code), "\n")
				if len(lines) >= 2 && strings.TrimSpace(lines[0]) == "{" && strings.TrimSpace(lines[len(lines)-1]) == "}" {
					lines = lines[1 : len(lines)-1]
				}
				out = append(out, dedent(strings.Join(lines, "\n")))
			}
			return out
		}
		importPath := "pocketone.dev/" + filepath.ToSlash(dir)
		md := func(text string) string {
			return strings.TrimSpace(string(p.Markdown(text)))
		}
		comment := func(text string) string {
			var lines []string
			for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
				lines = append(lines, strings.TrimRight("// "+l, " "))
			}
			if len(lines) == 1 && lines[0] == "//" {
				return ""
			}
			return strings.Join(lines, "\n")
		}
		addFunc := func(name string, f *doc.Func) {
			decl := *f.Decl
			decl.Body, decl.Doc = nil, nil
			syms[name] = goSym{Sig: print(&decl), Doc: md(f.Doc), Comment: comment(f.Doc), Examples: examples(f.Examples), Import: importPath, Pkg: p.Name}
		}
		for _, f := range p.Funcs {
			addFunc(f.Name, f)
		}
		for _, t := range p.Types {
			for _, f := range t.Funcs {
				addFunc(f.Name, f)
			}
			for _, f := range t.Methods {
				addFunc(t.Name+"."+f.Name, f)
			}
			ts := t.Decl.Specs[0].(*ast.TypeSpec)
			sym := goSym{Doc: md(t.Doc), Examples: examples(t.Examples), Import: importPath, Pkg: p.Name}
			switch typ := ts.Type.(type) {
			case *ast.StructType:
				sym.Sig = "type " + t.Name + " struct"
				d := *t.Decl
				d.Doc = nil
				sym.Decl = print(&d)
				for _, f := range typ.Fields.List {
					for _, n := range f.Names {
						if !n.IsExported() {
							continue
						}
						d := strings.TrimSpace(f.Doc.Text())
						if d == "" {
							d = strings.TrimSpace(f.Comment.Text())
						}
						sym.Fields = append(sym.Fields, goField{Name: n.Name, Type: print(f.Type), Doc: d})
					}
				}
			case *ast.InterfaceType:
				sym.Sig = "type " + t.Name + " interface"
				for _, m := range typ.Methods.List {
					if len(m.Names) == 0 {
						continue
					}
					ft, ok := m.Type.(*ast.FuncType)
					if !ok {
						continue
					}
					sig := "func (" + t.Name + ") " + m.Names[0].Name + strings.TrimPrefix(print(ft), "func")
					syms[t.Name+"."+m.Names[0].Name] = goSym{Sig: sig, Doc: md(m.Doc.Text()), Comment: comment(m.Doc.Text()), Import: importPath, Pkg: p.Name}
				}
			default:
				sym.Sig = "type " + t.Name + " " + print(ts.Type)
			}
			syms[t.Name] = sym
		}
		out[p.Name] = syms
	}
	return out
}

// dedent removes the indentation every line shares.
func dedent(s string) string {
	lines := strings.Split(s, "\n")
	min := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if min < 0 || n < min {
			min = n
		}
	}
	for i, l := range lines {
		if len(l) >= min && min > 0 {
			lines[i] = l[min:]
		}
	}
	return strings.Join(lines, "\n")
}

// goDocText turns a Go doc comment's [Name] links into code.
func goDocText(s string) string {
	return regexp.MustCompile(`\[([A-Z][\w.]*)\]`).ReplaceAllString(strings.Join(strings.Fields(s), " "), "`$1`")
}

// goName is the name a Go signature declares: "Get" in "func Get(...)", "Store.Get" in "func (Store) Get(...)",
// "Entry" in "type Entry struct".
func goName(sig string) string {
	if m := regexp.MustCompile(`^func \((\w+)\) (\w+)`).FindStringSubmatch(sig); m != nil {
		return m[1] + "." + m[2]
	}
	if m := regexp.MustCompile(`^(?:func|type) (\w+)`).FindStringSubmatch(sig); m != nil {
		return m[1]
	}
	return sig
}

// lookupGo finds an @go target: "kvv1.Get" (a function), "kvv1.Store.Get" (a method), "kvv1.Entry" (a type).
func lookupGo(pkgs map[string]map[string]goSym, target string) (goSym, string, bool) {
	pkg, sym, ok := strings.Cut(target, ".")
	if !ok {
		return goSym{}, "", false
	}
	s, found := pkgs[pkg][sym]
	return s, pkg, found
}

// ---------------------------------------------------------------------------------------------------------------
// The page

// contractDirs is the folders of the contract packages an extension provides (ext/contracts/<id path>/vN).
func contractDirs(m *manifest, ext string) []string {
	var dirs []string
	ids := map[string]bool{}
	for _, id := range m.provides[ext] {
		ids[id] = true
	}
	for _, c := range m.contracts {
		if ids[c.ID] {
			dirs = append(dirs, filepath.Dir(c.Src))
		}
	}
	sort.Strings(dirs)
	return dirs
}

// typedPages is the extensions whose page is a typed API page (the experiment); the rest list their declarations.
var typedPages = map[string]bool{"kv": true}

func typedPage(root string, m *manifest, r jsRef) string {
	title := jsTitle(m, r.Extension)
	api := parseJSAPI(r.Types)
	pkgs := goPackages(root, contractDirs(m, r.Extension))
	var b strings.Builder
	b.WriteString(refFrontMatter(title+" API", "Every function and type "+title+" adds to pb_hooks and page code, and its Go counterpart: signatures, parameters, what each returns, and examples. Generated from the extension's TypeScript declarations and Go packages."))
	fmt.Fprintf(&b, "# %s API\n\n", title)
	fmt.Fprintf(&b, "Everything the [%s](/docs/extensions/%s) extension adds to `pb_hooks` and page code, each with its Go "+
		"counterpart in the TypeScript and Go tabs. Generated from the TypeScript declarations it ships (your app's `types.d.ts` "+
		"has the same) and its Go packages; for how to use it, see its docs.\n\n", title, r.Extension)
	if len(pkgs) > 0 {
		var imports []string
		for name, syms := range pkgs {
			for _, sy := range syms {
				imports = append(imports, fmt.Sprintf("`%s` (`%s`)", sy.Import, name))
				break
			}
		}
		sort.Strings(imports)
		fmt.Fprintf(&b, "The Go side is in %s: import it and pass your `core.App`.\n\n", strings.Join(imports, ", "))
	}
	for _, g := range api.Globals {
		entries := api.Entries[g.Name]
		if len(entries) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", g.Name)
		if d := api.GlobalDocs[g.Name]; d.Body != "" {
			b.WriteString(d.Body + "\n\n")
		}
		for _, e := range entries {
			writeEntry(&b, e, pkgs)
		}
	}
	if len(api.Types) > 0 {
		b.WriteString("## Types\n\n")
		for _, t := range api.Types {
			writeType(&b, t, pkgs)
		}
	}
	return b.String()
}

func writeEntry(b *strings.Builder, e apiEntry, pkgs map[string]map[string]goSym) {
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
	// the signatures, TypeScript and Go side by side
	fmt.Fprintf(b, "```ts\n%s\n```\n\n", e.Sig.display(prefix))
	var gos []goSym
	for _, t := range e.Doc.Go {
		if s, _, ok := lookupGo(pkgs, t); ok {
			gos = append(gos, s)
		}
	}
	if len(gos) > 0 {
		var sigs []string
		for _, s := range gos {
			// the Go doc comment over the signature, as Go shows its docs
			if s.Comment != "" {
				sigs = append(sigs, s.Comment+"\n"+s.Sig)
			} else {
				sigs = append(sigs, s.Sig)
			}
		}
		fmt.Fprintf(b, "```go\n%s\n```\n\n", strings.Join(sigs, "\n"))
	}
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
		fmt.Fprintf(b, "Returns `%s`", e.Sig.Returns)
		if e.Doc.Returns != "" {
			b.WriteString(": " + e.Doc.Returns)
		}
		b.WriteString(".\n\n")
	}

	// examples: each JS example beside the Go one in the same place
	var goEx []string
	for _, s := range gos {
		goEx = append(goEx, s.Examples...)
	}
	n := max(len(e.Doc.Examples), len(goEx))
	if n > 0 {
		b.WriteString("Example:\n\n")
	}
	for i := 0; i < n; i++ {
		if i < len(e.Doc.Examples) {
			fmt.Fprintf(b, "```ts\n%s\n```\n\n", e.Doc.Examples[i])
		}
		if i < len(goEx) {
			fmt.Fprintf(b, "```go\n%s\n```\n\n", goEx[i])
		}
	}
}

func writeType(b *strings.Builder, t apiType, pkgs map[string]map[string]goSym) {
	fmt.Fprintf(b, "### %s\n\n", t.Name)
	if t.Doc.Body != "" {
		b.WriteString(t.Doc.Body + "\n\n")
	}
	var gs []goSym
	for _, target := range t.Doc.Go {
		if s, _, ok := lookupGo(pkgs, target); ok {
			gs = append(gs, s)
		}
	}
	if len(t.Fields) > 0 {
		b.WriteString("| Field | Type | |\n|---|---|---|\n")
		for _, f := range t.Fields {
			d := f.Doc.Summary
			fmt.Fprintf(b, "| `%s` | `%s` | %s |\n", f.Sig.Name, strings.ReplaceAll(strings.TrimSuffix(f.Sig.Returns, " (optional)"), "|", "\\|"), cell(d))
		}
		b.WriteString("\n")
	} else if t.Text != "" {
		fmt.Fprintf(b, "```ts\ntype %s = %s\n```\n\n", t.Name, t.Text)
	}
	for _, s := range gs {
		if s.Decl != "" {
			fmt.Fprintf(b, "In Go, `%s.%s`:\n\n```go\n%s\n```\n\n", s.Pkg, goName(s.Sig), s.Decl)
		}
	}
}
