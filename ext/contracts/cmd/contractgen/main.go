// Command contractgen writes the code every PocketOne contract needs from its Go interface: the contract key, the
// guarded wrapper (scope check + action hooks), the routes' operations, typed hook helpers, the HTTP client and the JS
// adapter. Run it with go:generate in the contract's package:
//
//	//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen
//
//	// Deployer sends a project's code to an app.
//	//
//	//contract:id deploy/v1
//	//contract:kind wire
//	//contract:scope deploy
//	//contract:host
//	//contract:js hooks
//	type Deployer interface {
//		// Upload writes changed files. (becomes the MCP tool description)
//		Upload(ctx context.Context, a extkit.Auth, in UploadInput) (UploadResult, error)
//	}
//
// Directives:
//
//	contract:id <name>/v<N>           required
//	contract:kind wire|service        wire: an HTTP API at /api/_c/<id>/, action hooks, MCP tools, a client (default service)
//	contract:cardinality one|many|keyed   default one; wire contracts are always one
//	contract:key <Method>             keyed: the method returning each value's key (not an operation)
//	contract:scope <scope>            wire: the token scope every call needs
//	contract:host                     a multisite host may provide it for its instances
//	contract:js closed|hooks|open     what an app's pb_hooks may do with it (default closed)
//
// Methods take ctx first; a wire contract's take extkit.Auth second; then at most one input value. They return error,
// or one value and error. The generated files are zz_contract.go and, for a contract pb_hooks can touch (contract:js
// hooks or open), zz_contract.d.ts: its TypeScript declarations (ts.go), which zz_contract.go embeds for ext/extkit to
// add to the app's types.d.ts.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

type method struct {
	Name    string // Go name: Upload
	Op      string // operation: upload
	JSName  string // JS method name: upload
	Desc    string
	In      string // input type, "" when none
	Out     string // result type, "" when only error
	HookArg string // "in *UploadInput" or ""

	// wire operations
	HTTPMethod string // GET, POST, PUT, DELETE
	Path       string // under the contract's prefix ("" = the prefix)
	Scope      string // "" = the contract's
	RawIn      bool   // In is extkit.RawIn: the request body
	QueryIn    bool   // In comes from query parameters (GET, DELETE)
	OutKind    string // OutJSON, OutRaw, OutStream
}

type contract struct {
	Package     string
	Iface       string
	ID          string
	Wire        bool
	Cardinality string // One, Many, Keyed
	Key         string
	Scope       string
	Host        bool
	JS          string // JSClosed, JSHooks, JSOpen
	Methods     []method
	NeedsJSON   bool
	Imports     []string // extra imports the method types need ("ganigeorgiev.com/pocketbase/pocketbase/tools/mailer")
}

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	c, err := parse(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "contractgen:", err)
		os.Exit(1)
	}
	src, err := render(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "contractgen:", err)
		os.Exit(1)
	}
	ts, err := renderTS(dir, c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "contractgen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(dir, "zz_contract.go"), src, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "contractgen:", err)
		os.Exit(1)
	}
	tsPath := filepath.Join(dir, "zz_contract.d.ts")
	if ts == "" {
		_ = os.Remove(tsPath)
	} else if err := os.WriteFile(tsPath, []byte(ts), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "contractgen:", err)
		os.Exit(1)
	}
}

func parse(dir string) (*contract, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		n := fi.Name()
		return !strings.HasSuffix(n, "_test.go") && n != "zz_contract.go"
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var found *contract
	for pkgName, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, spec := range gd.Specs {
					ts := spec.(*ast.TypeSpec)
					doc := ts.Doc
					if doc == nil {
						doc = gd.Doc
					}
					dirs := directives(doc)
					if dirs["id"] == "" {
						continue
					}
					it, ok := ts.Type.(*ast.InterfaceType)
					if !ok {
						return nil, fmt.Errorf("%s: contract:id on a non-interface type", ts.Name.Name)
					}
					if found != nil {
						return nil, fmt.Errorf("one contract per package (%s and %s)", found.Iface, ts.Name.Name)
					}
					c, err := build(pkgName, ts.Name.Name, dirs, it)
					if err != nil {
						return nil, fmt.Errorf("%s: %w", ts.Name.Name, err)
					}
					c.Imports = neededImports(f, c)
					found = c
				}
			}
		}
	}
	if found == nil {
		return nil, fmt.Errorf("no interface with a //contract:id directive in %s", dir)
	}
	return found, nil
}

// neededImports are the file's imports that the methods' types use (besides context and extkit, always imported).
func neededImports(f *ast.File, c *contract) []string {
	byName := map[string]string{}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		name := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		byName[name] = path
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range c.Methods {
		for _, t := range []string{m.In, m.Out} {
			for name, path := range byName {
				if path == "context" || strings.HasSuffix(path, "/ext/extkit") || seen[path] {
					continue
				}
				if strings.Contains(t, name+".") {
					seen[path] = true
					spec := fmt.Sprintf("%q", path)
					if name != path[strings.LastIndex(path, "/")+1:] {
						spec = name + " " + spec
					}
					out = append(out, spec)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func directives(doc *ast.CommentGroup) map[string]string {
	out := map[string]string{}
	if doc == nil {
		return out
	}
	for _, c := range doc.List {
		rest, ok := strings.CutPrefix(c.Text, "//contract:")
		if !ok {
			continue
		}
		k, v, _ := strings.Cut(strings.TrimSpace(rest), " ")
		out[k] = strings.TrimSpace(v)
		if v == "" {
			out[k] = "true"
		}
	}
	return out
}

func build(pkg, iface string, d map[string]string, it *ast.InterfaceType) (*contract, error) {
	c := &contract{Package: pkg, Iface: iface, ID: d["id"], Scope: d["scope"], Host: d["host"] == "true", Key: d["key"]}
	if !strings.Contains(c.ID, "/v") {
		return nil, fmt.Errorf("contract:id %q must be <name>/v<N>", c.ID)
	}
	switch d["kind"] {
	case "", "service":
	case "wire":
		c.Wire = true
	default:
		return nil, fmt.Errorf("contract:kind %q: want wire or service", d["kind"])
	}
	switch d["cardinality"] {
	case "", "one":
		c.Cardinality = "One"
	case "many":
		c.Cardinality = "Many"
	case "keyed":
		c.Cardinality = "Keyed"
		if c.Key == "" {
			return nil, fmt.Errorf("keyed contracts name their key method: //contract:key Name")
		}
	default:
		return nil, fmt.Errorf("contract:cardinality %q: want one, many or keyed", d["cardinality"])
	}
	switch d["js"] {
	case "", "closed":
		c.JS = "JSClosed"
	case "hooks":
		c.JS = "JSHooks"
	case "open":
		c.JS = "JSOpen"
	default:
		return nil, fmt.Errorf("contract:js %q: want closed, hooks or open", d["js"])
	}
	if c.Wire {
		if c.Cardinality != "One" {
			return nil, fmt.Errorf("wire contracts are one: there'd be no single provider to answer")
		}
		if c.Scope == "" {
			return nil, fmt.Errorf("wire contracts name the token scope their calls need: //contract:scope <name>")
		}
		if !validScope(c.Scope) {
			return nil, fmt.Errorf("contract:scope %q: API token scopes are lower-case letters, digits, \":\" and \"_\" (no dots)", c.Scope)
		}
	} else if c.Host {
		return nil, fmt.Errorf("only wire contracts can be host-provided (the host answers them over HTTP)")
	}
	if c.JS == "JSHooks" && !c.Wire {
		return nil, fmt.Errorf("contract:js hooks needs a wire contract: service contracts have no action hooks")
	}

	for _, f := range it.Methods.List {
		ft, ok := f.Type.(*ast.FuncType)
		if !ok || len(f.Names) != 1 {
			return nil, fmt.Errorf("contract interfaces list methods only (no embedding)")
		}
		name := f.Names[0].Name
		if name == c.Key {
			continue
		}
		m, err := buildMethod(name, ft, c.Wire)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		desc, op := splitOp(f.Doc)
		m.Desc = describe(name, desc)
		if c.Wire {
			if err := wireOp(&m, op); err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
		} else if op != "" {
			return nil, fmt.Errorf("%s: //op: is for wire contracts", name)
		}
		if c.Wire && m.Desc == "" {
			return nil, fmt.Errorf("%s: document it; the comment is the MCP tool's description", name)
		}
		c.Methods = append(c.Methods, m)
	}
	if len(c.Methods) == 0 {
		return nil, fmt.Errorf("no methods")
	}
	c.NeedsJSON = c.Wire || c.JS == "JSOpen"
	return c, nil
}

// splitOp separates a method's doc comment from its `//op: METHOD path [scope=x]` line.
func splitOp(doc *ast.CommentGroup) (text, op string) {
	if doc == nil {
		return "", ""
	}
	var lines []string
	for _, c := range doc.List {
		if rest, ok := strings.CutPrefix(c.Text, "//op:"); ok {
			op = strings.TrimSpace(rest)
			continue
		}
		lines = append(lines, strings.TrimSpace(strings.TrimPrefix(c.Text, "//")))
	}
	return strings.Join(lines, "\n"), op
}

// wireOp fills a wire method's HTTP side: its //op: line (default POST <op>), and the input and result kinds its types
// imply (extkit.RawIn, extkit.RawOut, extkit.Stream).
func wireOp(m *method, op string) error {
	m.HTTPMethod, m.Path, m.OutKind = "POST", m.Op, "OutJSON"
	if op != "" {
		f := strings.Fields(op)
		m.HTTPMethod = strings.ToUpper(f[0])
		m.Path = ""
		for _, x := range f[1:] {
			if v, ok := strings.CutPrefix(x, "scope="); ok {
				if !validScope(v) {
					return fmt.Errorf("//op: %q: scope %q: API token scopes are lower-case letters, digits, \":\" and \"_\" (no dots)", op, v)
				}
				m.Scope = v
			} else if m.Path == "" {
				m.Path = strings.Trim(x, "/")
			} else {
				return fmt.Errorf("//op: %q: unexpected %q", op, x)
			}
		}
		switch m.HTTPMethod {
		case "GET", "POST", "PUT", "DELETE", "PATCH":
		default:
			return fmt.Errorf("//op: %q: method %s", op, m.HTTPMethod)
		}
	}
	switch m.In {
	case "extkit.RawIn":
		m.RawIn = true
		if m.HTTPMethod == "GET" || m.HTTPMethod == "DELETE" {
			return fmt.Errorf("an upload (extkit.RawIn) needs POST or PUT")
		}
	case "":
	default:
		m.QueryIn = m.HTTPMethod == "GET" || m.HTTPMethod == "DELETE"
	}
	switch m.Out {
	case "extkit.RawOut":
		m.OutKind = "OutRaw"
	case "extkit.Stream":
		m.OutKind = "OutStream"
		if m.HTTPMethod != "GET" && m.HTTPMethod != "POST" {
			return fmt.Errorf("a stream (extkit.Stream) is a GET (input in the query) or a POST (input as a JSON body)")
		}
	}
	return nil
}

func buildMethod(name string, ft *ast.FuncType, wire bool) (method, error) {
	m := method{Name: name, Op: snake(name), JSName: lowerFirst(name)}
	var params []string
	for _, p := range ft.Params.List {
		t := types.ExprString(p.Type)
		n := len(p.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			params = append(params, t)
		}
	}
	want := []string{"context.Context"}
	if wire {
		want = append(want, "extkit.Auth")
	}
	if len(params) < len(want) || len(params) > len(want)+1 {
		return m, fmt.Errorf("takes (%s[, input]); got (%s)", strings.Join(want, ", "), strings.Join(params, ", "))
	}
	for i, w := range want {
		if params[i] != w {
			return m, fmt.Errorf("parameter %d must be %s, got %s", i+1, w, params[i])
		}
	}
	if len(params) == len(want)+1 {
		m.In = params[len(want)]
		m.HookArg = "in *" + m.In
	}
	var results []string
	if ft.Results != nil {
		for _, r := range ft.Results.List {
			results = append(results, types.ExprString(r.Type))
		}
	}
	switch {
	case len(results) == 1 && results[0] == "error":
	case len(results) == 2 && results[1] == "error":
		m.Out = results[0]
	default:
		return m, fmt.Errorf("returns error or (value, error); got (%s)", strings.Join(results, ", "))
	}
	return m, nil
}

// describe turns a Go doc comment ("Upload writes changed files.") into a tool description ("Writes changed files.").
func describe(name, doc string) string {
	d := strings.Join(strings.Fields(doc), " ")
	if rest, ok := strings.CutPrefix(d, name+" "); ok && rest != "" {
		r := []rune(rest)
		r[0] = unicode.ToUpper(r[0])
		d = string(r)
	}
	return d
}

func snake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func lowerFirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func render(c *contract) ([]byte, error) {
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, c); err != nil {
		return nil, err
	}
	out, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("generated code doesn't format (a contractgen bug): %w", err)
	}
	return out, nil
}

var tpl = template.Must(template.New("").Funcs(template.FuncMap{
	"lower": lowerFirst,
}).Parse(`// Code generated by contractgen from {{.Iface}}. DO NOT EDIT.

package {{.Package}}

import (
	"context"
{{- if ne .JS "JSClosed"}}
	_ "embed"
{{- end}}
{{- if .NeedsJSON}}
	"encoding/json"
{{- end}}
{{- if .Wire}}
	"net/http"
	"reflect"
{{- end}}

	"pocketone.dev/ext/extkit"
{{- if or .Wire (eq .JS "JSOpen")}}{{/* only generated method code uses the methods' types */}}
{{- range .Imports}}
	{{.}}
{{- end}}
{{- end}}
)

var (
	_ context.Context
{{- if .Wire}}
	_ reflect.Type
{{- end}}
{{- if .NeedsJSON}}
	_ json.RawMessage
{{- end}}
)

// Contract is {{.ID}}.
var Contract = {{if eq .Cardinality "Keyed"}}extkit.NewKeyedContract[{{.Iface}}]("{{.ID}}", func(v {{.Iface}}) string { return v.{{.Key}}() }){{else}}extkit.NewContract[{{.Iface}}]("{{.ID}}", extkit.{{.Cardinality}}){{end}}
{{- if .Host}}.HostProvidable(){{end}}
{{- if ne .JS "JSClosed"}}.JS(extkit.{{.JS}}, {{if eq .JS "JSOpen"}}func(o *extkit.JSObject) {{.Iface}} { return js{{.Iface}}{o} }{{else}}nil{{end}}).JSTypes(jsTypes){{end}}
{{- if ne .JS "JSClosed"}}

// jsTypes is {{.ID}}'s TypeScript declarations for pb_hooks (zz_contract.d.ts), added to the app's types.d.ts.
//
//go:embed zz_contract.d.ts
var jsTypes string
{{- end}}

{{- if .Wire}}

// W is {{.ID}}'s wire side: routes at /api/_c/{{.ID}}/<op>, action hooks, MCP tools and the client.
var W = extkit.NewWire(Contract, "{{.Scope}}", func(inner {{.Iface}}, g *extkit.Guard[{{.Iface}}]) {{.Iface}} {
	return &guard{{.Iface}}{inner, g}
},
{{- range .Methods}}
	extkit.Op[{{$.Iface}}]{
		Name:   "{{.Op}}",
		Desc:   {{printf "%q" .Desc}},
		Method: "{{.HTTPMethod}}",
		Path:   "{{.Path}}",
		{{- if .Scope}}
		Scope:  "{{.Scope}}",
		{{- end}}
		{{- if .RawIn}}
		RawIn:  true,
		{{- else if .In}}
		In:     reflect.TypeOf({{.In}}{}),
		{{- end}}
		Out:    extkit.{{.OutKind}},
		Invoke: func(ctx context.Context, impl {{$.Iface}}, a extkit.Auth, req extkit.OpRequest) (any, error) {
		{{- if .RawIn}}
			in := extkit.RawIn{Query: req.Query, Body: req.Body, ContentType: req.ContentType, Header: req.Header}
		{{- else if .QueryIn}}
			var in {{.In}}
			if err := extkit.DecodeQuery(req.Query, &in); err != nil {
				return nil, err
			}
		{{- else if .In}}
			var in {{.In}}
			if err := extkit.DecodeInput(req.JSON, &in); err != nil {
				return nil, err
			}
		{{- end}}
		{{- if .Out}}
			return impl.{{.Name}}(ctx, a{{if .In}}, in{{end}})
		{{- else}}
			return nil, impl.{{.Name}}(ctx, a{{if .In}}, in{{end}})
		{{- end}}
		},
	},
{{- end}}
)

// guard{{.Iface}} is the outermost layer: every call, from any surface, passes the scope check and the action hooks.
type guard{{.Iface}} struct {
	inner {{.Iface}}
	g     *extkit.Guard[{{.Iface}}]
}
{{range .Methods}}
func (d *guard{{$.Iface}}) {{.Name}}(ctx context.Context, a extkit.Auth{{if .In}}, in {{.In}}{{end}}) ({{if .Out}}{{.Out}}, {{end}}error) {
{{- if not .In}}
	in := struct{}{}
{{- end}}
	{{if .Out}}return{{else}}_, err :={{end}} extkit.Run(d.g, a, "{{.Op}}", &in, func(in *{{if .In}}{{.In}}{{else}}struct{}{{end}}) ({{if .Out}}{{.Out}}{{else}}struct{}{{end}}, error) {
		{{if .Out}}return d.inner.{{.Name}}(ctx, a{{if .In}}, *in{{end}}){{else}}return struct{}{}, d.inner.{{.Name}}(ctx, a{{if .In}}, *in{{end}}){{end}}
	})
{{- if not .Out}}
	return err
{{- end}}
}

// On{{.Name}} binds fn to {{.Op}} calls on owner (an app, or a host's scope for calls the host answers). Call e.Next()
// to continue; return an error to refuse.{{if .In}} in is the call's input; changes reach the provider.{{end}}
func On{{.Name}}(owner any, fn func(e *extkit.CallEvent{{if .In}}, in *{{.In}}{{end}}) error) {
	W.On(owner, "{{.Op}}").BindFunc(func(e *extkit.CallEvent) error {
		return fn(e{{if .In}}, e.Input.(*{{.In}}){{end}})
	})
}
{{end}}
// Client is {{.ID}} over HTTP, for the CLI and other apps. It implements {{.Iface}}; the token it was made with is
// the credential, so the extkit.Auth argument is ignored.
type Client struct{ c *extkit.Caller }

// NewClient calls {{.ID}} on the app at baseURL with token (a PocketBase auth token or an API token).
func NewClient(baseURL, token string, hc *http.Client) *Client {
	return &Client{W.Client(baseURL, token, hc)}
}

var _ {{.Iface}} = (*Client)(nil)
{{range .Methods}}
func (c *Client) {{.Name}}(ctx context.Context, _ extkit.Auth{{if .In}}, in {{.In}}{{end}}) ({{if .Out}}{{.Out}}, {{end}}error) {
{{- if eq .OutKind "OutRaw"}}
	return c.c.CallRawOut(ctx, "{{.HTTPMethod}}", "{{.Path}}", {{if .In}}in{{else}}nil{{end}})
{{- else if eq .OutKind "OutStream"}}
	return c.c.CallStreamMethod(ctx, "{{.HTTPMethod}}", "{{.Path}}", {{if .In}}in{{else}}nil{{end}})
{{- else if .RawIn}}
{{- if .Out}}
	var out {{.Out}}
	return out, c.c.CallRawIn(ctx, "{{.HTTPMethod}}", "{{.Path}}", in, &out)
{{- else}}
	return c.c.CallRawIn(ctx, "{{.HTTPMethod}}", "{{.Path}}", in, nil)
{{- end}}
{{- else if .Out}}
	var out {{.Out}}
	return out, c.c.Call(ctx, "{{.HTTPMethod}}", "{{.Path}}", {{if .In}}in{{else}}struct{}{}{{end}}, &out)
{{- else}}
	return c.c.Call(ctx, "{{.HTTPMethod}}", "{{.Path}}", {{if .In}}in{{else}}struct{}{}{{end}}, nil)
{{- end}}
}
{{end}}
{{- end}}

{{- if eq .JS "JSOpen"}}

// js{{.Iface}} is a provider written in pb_hooks: onContracts((e) => e.provide("{{.ID}}", { {{range $i, $m := .Methods}}{{if $i}}, {{end}}{{.JSName}}({{if .In}}in{{end}}{{if $.Wire}}{{if .In}}, {{end}}auth{{end}}) {...}{{end}} })).
// Inputs and results cross as JSON, so JS sees their json field names.
type js{{.Iface}} struct{ o *extkit.JSObject }
{{if eq .Cardinality "Keyed"}}
// {{.Key}} is the JS object's {{lower .Key}}(): its key.
func (j js{{.Iface}}) {{.Key}}() string {
	var k string
	_ = j.o.CallJSON(context.Background(), "{{lower .Key}}", &k)
	return k
}
{{end}}{{range .Methods}}
func (j js{{$.Iface}}) {{.Name}}(ctx context.Context{{if $.Wire}}, a extkit.Auth{{end}}{{if .In}}, in {{.In}}{{end}}) ({{if .Out}}{{.Out}}, {{end}}error) {
{{- if .Out}}
	var out {{.Out}}
	return out, j.o.CallJSON(ctx, "{{.JSName}}", &out{{if .In}}, in{{end}}{{if $.Wire}}, a{{end}})
{{- else}}
	return j.o.CallJSON(ctx, "{{.JSName}}", nil{{if .In}}, in{{end}}{{if $.Wire}}, a{{end}})
{{- end}}
}
{{end}}
{{- end}}
`))

// validScope reports whether s may name an API token scope (ext/api-tokens' rule): lower-case letters, digits, ":"
// and "_", starting with a letter or digit; "public" means no scope.
func validScope(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || i > 0 && (r == ':' || r == '_')
		if !ok {
			return false
		}
	}
	return true
}
