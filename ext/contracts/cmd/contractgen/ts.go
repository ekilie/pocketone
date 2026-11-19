package main

// TypeScript declarations (zz_contract.d.ts) for a contract pb_hooks can touch (contract:js hooks or open), from its
// type-checked Go package. ext/extkit embeds each one in the app's types.d.ts.
//
// Two views of a Go type, because JS meets a contract's values two ways:
//
//   - Hooks (a wire contract's on<Contract><Op>): e.input is the Go value itself, so fields carry ext/jsvm's names
//     (Go name, first letter lowered: MaxTokens → maxTokens, ID → id) and json tags don't matter.
//   - Providers (contract:js open, e.provide): arguments and results cross as JSON (JSObject.CallJSON), so fields
//     carry their json names, omitempty fields are optional, and time.Time is a string.
//
// The declarations are self-contained: they use only the contracts namespace ext/extkit declares, never the engine's
// Go package declarations, so tsc checks them on their own.

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode"
)

type tsView int

const (
	viewGo   tsView = iota // a hook's e.input: ext/jsvm field names
	viewJSON               // a JS provider's arguments and results: json names
)

// typedPackage type-checks the contract package in dir. Its imports come from the compiler's export data (go list
// -export), which the build cache keeps, so this takes well under a second. Errors in the package itself (a stale
// zz_contract.go) don't matter: only the interface and its types are needed.
func typedPackage(dir string) (*types.Package, map[string]*ast.File, *token.FileSet, error) {
	cmd := exec.Command("go", "list", "-e", "-export", "-deps", "-f", "{{.ImportPath}}={{.Export}}", ".")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, nil, nil, fmt.Errorf("go list -export: %v\n%s", err, ee.Stderr)
		}
		return nil, nil, nil, fmt.Errorf("go list -export: %v", err)
	}
	export := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), "="); ok && v != "" {
			export[k] = v
		}
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, nil, nil, err
	}
	for name, p := range pkgs {
		var files []*ast.File
		var names []string
		for fn := range p.Files {
			names = append(names, fn)
		}
		sort.Strings(names)
		for _, fn := range names {
			files = append(files, p.Files[fn])
		}
		conf := types.Config{
			Importer: importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
				f := export[path]
				if f == "" {
					return nil, fmt.Errorf("no export data for %s", path)
				}
				return os.Open(f)
			}),
			Error: func(error) {},
		}
		pkg, _ := conf.Check(name, fset, files, nil)
		return pkg, p.Files, fset, nil
	}
	return nil, nil, nil, fmt.Errorf("no Go package in %s", dir)
}

// renderTS returns zz_contract.d.ts for c, or "" when pb_hooks can't touch it (contract:js closed).
func renderTS(dir string, c *contract) (string, error) {
	if c.JS == "JSClosed" {
		return "", nil
	}
	pkg, files, _, err := typedPackage(dir)
	if err != nil {
		return "", err
	}
	obj := pkg.Scope().Lookup(c.Iface)
	if obj == nil {
		return "", fmt.Errorf("%s isn't in the type-checked package", c.Iface)
	}
	iface, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		return "", fmt.Errorf("%s isn't an interface", c.Iface)
	}
	g := &tsGen{pkg: pkg, docs: collectDocs(files), names: map[tsKey]string{}, taken: map[string]bool{}}
	g.both = c.Wire && c.JS == "JSOpen"
	sigs := map[string]*types.Signature{}
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		sigs[m.Name()] = m.Type().(*types.Signature)
	}
	ns := "contracts." + tsNamespace(c.ID)

	var hooks strings.Builder
	if c.Wire {
		for _, m := range c.Methods {
			in := "{}"
			if m.In != "" {
				sig := sigs[m.Name]
				g.qual = ns + "."
				in = g.ref(deref(sig.Params().At(sig.Params().Len()-1).Type()), viewGo)
				g.qual = ""
			}
			hooks.WriteString("/**\n")
			if d := g.docs[c.Iface+"."+m.Name]; d != "" {
				hooks.WriteString(jsdocLines(d, " ") + " *\n")
			}
			fmt.Fprintf(&hooks, " * Runs on every %s call to %s, from any surface (HTTP, Go, MCP, the CLI): change e.input, or throw to\n * refuse.\n *\n * @group PocketOne\n */\n", m.Op, c.ID)
			fmt.Fprintf(&hooks, "declare function %s(handler: (e: contracts.CallEvent<%s>) => void): void;\n", hookName(c.ID, m.Op), in)
		}
	}

	var provider strings.Builder
	if c.JS == "JSOpen" {
		fmt.Fprintf(&provider, "  /**\n%s   *\n   * What e.provide(%q, impl) takes in pb_hooks: methods may be async; throw ContractError to fail.\n   */\n",
			jsdocLines(g.docs[c.Iface], "   "), c.ID)
		fmt.Fprintf(&provider, "  interface %s {\n", c.Iface)
		if c.Key != "" {
			fmt.Fprintf(&provider, "    /** This provider's key. */\n    %s(): string;\n", lowerFirst(c.Key))
		}
		for _, m := range c.Methods {
			sig := sigs[m.Name]
			var params []string
			if m.In != "" {
				p := sig.Params().At(sig.Params().Len() - 1)
				name := p.Name()
				if name == "" || name == "_" {
					name = "in"
				}
				params = append(params, name+": "+g.ref(deref(p.Type()), viewJSON))
			}
			if c.Wire {
				params = append(params, "auth: contracts.Auth")
			}
			ret := "void"
			if m.Out != "" {
				ret = g.ref(deref(sig.Results().At(0).Type()), viewJSON)
			}
			if d := g.docs[c.Iface+"."+m.Name]; d != "" {
				fmt.Fprintf(&provider, "    /**\n%s     */\n", jsdocLines(d, "     "))
			}
			fmt.Fprintf(&provider, "    %s(%s): %s | Promise<%s>;\n", m.JSName, strings.Join(params, ", "), ret, ret)
		}
		provider.WriteString("  }\n")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by contractgen from %s. DO NOT EDIT.\n\n", c.Iface)
	fmt.Fprintf(&b, "/** %s: the types pb_hooks sees. */\n", c.ID)
	fmt.Fprintf(&b, "declare namespace %s {\n", ns)
	b.WriteString(strings.TrimPrefix(provider.String()+g.decls(), "\n"))
	b.WriteString("}\n")
	if c.JS == "JSOpen" {
		fmt.Fprintf(&b, "\ndeclare namespace contracts {\n  interface Providers {\n    %q: %s.%s;\n  }\n}\n", c.ID, ns, c.Iface)
	}
	if hooks.Len() > 0 {
		b.WriteString("\n")
		b.WriteString(hooks.String())
	}
	return b.String(), nil
}

// tsGen collects the named types a contract's declarations use, each declared once in the contract's namespace.
type tsGen struct {
	pkg   *types.Package
	docs  map[string]string
	both  bool // the contract needs both views: JSON-view names get a JSON suffix
	names map[tsKey]string
	taken map[string]bool
	qual  string // prefix for named types (the namespace, outside it)
	queue []tsNamed
	out   strings.Builder
}

type tsKey struct {
	tn   *types.TypeName
	view tsView
}

type tsNamed struct {
	tn   *types.TypeName
	st   *types.Struct
	view tsView
	name string
}

// ref is the TypeScript for t in view. Named structs are declared in the namespace and referenced by name.
func (g *tsGen) ref(t types.Type, view tsView) string {
	switch t := t.(type) {
	case *types.Alias:
		return g.ref(types.Unalias(t), view)
	case *types.Named:
		tn := t.Obj()
		path := ""
		if tn.Pkg() != nil {
			path = tn.Pkg().Path()
		}
		full := path + "." + tn.Name()
		if view == viewJSON {
			switch {
			case full == "encoding/json.RawMessage":
				return "any"
			case full == "time.Time", full == "ganigeorgiev.com/pocketbase/pocketbase/tools/types.DateTime":
				return "string"
			case hasMethod(t, "MarshalJSON"):
				return "any"
			case hasMethod(t, "MarshalText"):
				return "string"
			}
		}
		if st, ok := t.Underlying().(*types.Struct); ok {
			if !hasExportedField(st) {
				return "any" // opaque to JS (time.Time on the Go side)
			}
			return g.named(tn, st, view)
		}
		return g.ref(t.Underlying(), view)
	case *types.Basic:
		switch {
		case t.Info()&types.IsString != 0:
			return "string"
		case t.Info()&types.IsBoolean != 0:
			return "boolean"
		case t.Info()&types.IsNumeric != 0:
			return "number"
		}
		return "any"
	case *types.Pointer:
		return g.ref(t.Elem(), view) + " | null"
	case *types.Slice:
		if b, ok := t.Elem().Underlying().(*types.Basic); ok && b.Kind() == types.Byte {
			if view == viewJSON {
				return "string" // base64
			}
			return "any"
		}
		return "Array<" + g.ref(t.Elem(), view) + ">"
	case *types.Array:
		return "Array<" + g.ref(t.Elem(), view) + ">"
	case *types.Map:
		return "Record<string, " + g.ref(t.Elem(), view) + ">"
	case *types.Struct:
		var b strings.Builder
		b.WriteString("{ ")
		for _, f := range g.fields(t, view) {
			fmt.Fprintf(&b, "%s%s: %s; ", propKey(f.name), opt(f.optional), f.ts)
		}
		b.WriteString("}")
		return b.String()
	}
	return "any" // interfaces, funcs, channels, type parameters
}

// named declares tn (once per view) and returns its TypeScript name.
func (g *tsGen) named(tn *types.TypeName, st *types.Struct, view tsView) string {
	key := tsKey{tn, view}
	if n, ok := g.names[key]; ok {
		return g.qual + n
	}
	name := tn.Name()
	if tn.Pkg() != nil && tn.Pkg() != g.pkg {
		name = upperFirst(tn.Pkg().Name()) + name // mailer.Message → MailerMessage
	}
	if view == viewJSON && g.both {
		name += "JSON"
	}
	for base, i := name, 2; g.taken[name]; i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	g.taken[name] = true
	g.names[key] = name
	g.queue = append(g.queue, tsNamed{tn: tn, st: st, view: view, name: name})
	return g.qual + name
}

// decls writes every queued type (declaring a type may queue more).
func (g *tsGen) decls() string {
	for i := 0; i < len(g.queue); i++ {
		n := g.queue[i]
		docKey := n.tn.Name()
		own := n.tn.Pkg() == g.pkg
		g.out.WriteString("\n")
		if d := g.docs[docKey]; own && d != "" {
			fmt.Fprintf(&g.out, "  /**\n%s   */\n", jsdocLines(d, "   "))
		}
		fmt.Fprintf(&g.out, "  interface %s {\n", n.name)
		for _, f := range g.fields(n.st, n.view) {
			if d := g.docs[docKey+"."+f.goName]; own && d != "" {
				fmt.Fprintf(&g.out, "    /**\n%s     */\n", jsdocLines(d, "     "))
			}
			fmt.Fprintf(&g.out, "    %s%s: %s;\n", propKey(f.name), opt(f.optional), f.ts)
		}
		g.out.WriteString("  }\n")
	}
	return g.out.String()
}

type tsField struct {
	name, goName, ts string
	optional         bool
}

// fields lists st's fields as JS sees them in view.
func (g *tsGen) fields(st *types.Struct, view tsView) []tsField {
	var out []tsField
	seen := map[string]bool{}
	add := func(f tsField) {
		if !seen[f.name] {
			seen[f.name] = true
			out = append(out, f)
		}
	}
	for i := 0; i < st.NumFields(); i++ {
		v := st.Field(i)
		if view == viewGo {
			// ext/jsvm: exported fields, promoted ones flattened (reflect.VisibleFields), embedded ones themselves hidden
			if v.Embedded() {
				if est := embeddedStruct(v.Type()); est != nil {
					for _, f := range g.fields(est, view) {
						add(f)
					}
				}
				continue
			}
			if !v.Exported() {
				continue
			}
			add(tsField{name: jsvmName(v.Name()), goName: v.Name(), ts: g.ref(v.Type(), view)})
			continue
		}
		// encoding/json
		tag := reflectTag(st.Tag(i), "json")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "-" && opts == "" {
			continue
		}
		if v.Embedded() && name == "" {
			if est := embeddedStruct(v.Type()); est != nil {
				for _, f := range g.fields(est, view) {
					add(f)
				}
				continue
			}
		}
		if !v.Exported() {
			continue
		}
		if name == "" {
			name = v.Name()
		}
		f := tsField{name: name, goName: v.Name(), ts: g.ref(v.Type(), view)}
		for _, o := range strings.Split(opts, ",") {
			switch o {
			case "omitempty", "omitzero":
				f.optional = true
			case "string":
				f.ts = "string"
			}
		}
		add(f)
	}
	return out
}

// deref drops a parameter's or result's pointer: a method's argument is never nil.
func deref(t types.Type) types.Type {
	if p, ok := t.(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

func embeddedStruct(t types.Type) *types.Struct {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	st, _ := t.Underlying().(*types.Struct)
	return st
}

func hasExportedField(st *types.Struct) bool {
	for i := 0; i < st.NumFields(); i++ {
		if st.Field(i).Exported() {
			return true
		}
		if st.Field(i).Embedded() {
			if est := embeddedStruct(st.Field(i).Type()); est != nil && hasExportedField(est) {
				return true
			}
		}
	}
	return false
}

func hasMethod(t types.Type, name string) bool {
	ms := types.NewMethodSet(types.NewPointer(t))
	for i := 0; i < ms.Len(); i++ {
		if ms.At(i).Obj().Name() == name {
			return true
		}
	}
	return false
}

// reflectTag is reflect.StructTag.Get, for a tag string from go/types.
func reflectTag(tag, key string) string {
	for tag != "" {
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			return ""
		}
		name := tag[:i]
		tag = tag[i+1:]
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			return ""
		}
		qv := tag[:i+1]
		tag = tag[i+1:]
		if name == key {
			v := strings.Trim(qv, `"`)
			return strings.ReplaceAll(v, `\"`, `"`)
		}
	}
	return ""
}

// collectDocs maps "Type" and "Type.Field" (and "Iface.Method") to their doc comments, in the contract's own package.
func collectDocs(files map[string]*ast.File) map[string]string {
	docs := map[string]string{}
	text := func(groups ...*ast.CommentGroup) string {
		var parts []string
		for _, g := range groups {
			if g == nil {
				continue
			}
			var lines []string
			for _, c := range g.List {
				t := strings.TrimPrefix(c.Text, "//")
				if strings.HasPrefix(t, "contract:") || strings.HasPrefix(t, "op:") || strings.HasPrefix(t, "go:") {
					continue
				}
				lines = append(lines, strings.TrimPrefix(t, " "))
			}
			if s := strings.TrimSpace(strings.Join(lines, "\n")); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n")
	}
	for _, f := range files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				doc := ts.Doc
				if doc == nil && len(gd.Specs) == 1 {
					doc = gd.Doc
				}
				docs[ts.Name.Name] = text(doc)
				var list *ast.FieldList
				switch t := ts.Type.(type) {
				case *ast.StructType:
					list = t.Fields
				case *ast.InterfaceType:
					list = t.Methods
				}
				if list == nil {
					continue
				}
				for _, fl := range list.List {
					for _, n := range fl.Names {
						docs[ts.Name.Name+"."+n.Name] = text(fl.Doc, fl.Comment)
					}
				}
			}
		}
	}
	return docs
}

func jsdocLines(s, indent string) string {
	var b strings.Builder
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		l = strings.ReplaceAll(l, "*/", "*\\/")
		if l == "" {
			b.WriteString(indent + "*\n")
		} else {
			b.WriteString(indent + "* " + l + "\n")
		}
	}
	return b.String()
}

func opt(o bool) string {
	if o {
		return "?"
	}
	return ""
}

// propKey quotes a property name that isn't a JS identifier.
func propKey(s string) string {
	for i, r := range s {
		if !(r == '_' || r == '$' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r))) {
			return fmt.Sprintf("%q", s)
		}
	}
	if s == "" {
		return `""`
	}
	return s
}

// tsNamespace is the contract's namespace under contracts: mail.fallback/v1 → mail.fallback.v1, api-tokens/v1 →
// apiTokens.v1.
func tsNamespace(id string) string {
	name, ver, _ := strings.Cut(id, "/")
	var parts []string
	for _, p := range append(strings.Split(name, "."), ver) {
		var b strings.Builder
		up := false
		for i, r := range p {
			switch {
			case r == '-' || r == '_':
				up = true
			case up:
				b.WriteRune(unicode.ToUpper(r))
				up = false
			case i == 0 && unicode.IsDigit(r):
				b.WriteRune('_')
				b.WriteRune(r)
			default:
				b.WriteRune(r)
			}
		}
		s := b.String()
		if tsReserved[s] {
			s += "_"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ".")
}

var tsReserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`break case catch class const continue debugger default delete do else enum export
		extends false finally for function if import in instanceof new null return super switch this throw true try
		typeof var void while with implements interface let package private protected public static yield`) {
		tsReserved[w] = true
	}
}

// hookName is ext/extkit's jsHookName: deploy/v1 + upload → onDeployUpload.
func hookName(id, op string) string {
	base := strings.SplitN(id, "/", 2)[0]
	var b strings.Builder
	b.WriteString("on")
	for _, part := range strings.FieldsFunc(base+"."+op, func(r rune) bool { return r == '.' || r == '_' || r == '-' }) {
		r := []rune(part)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}

// jsvmName is ext/jsvm's name for a Go field: MaxTokens → maxTokens, ID → id, HTMLBody → htmlBody.
func jsvmName(name string) string {
	if name == "OAuth2" {
		return "oauth2"
	}
	upper := 0
	for _, c := range name {
		if c != '_' && !unicode.IsUpper(c) && !unicode.IsDigit(c) {
			break
		}
		upper++
	}
	switch {
	case upper == len(name):
		return strings.ToLower(name)
	case upper > 1:
		return strings.ToLower(name[0:upper-1]) + name[upper-1:]
	case upper == 1:
		return strings.ToLower(name[0:1]) + name[1:]
	}
	return name
}

func upperFirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
