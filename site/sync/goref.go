package main

// The Go API reference (/docs/reference/go and a page per package under it): every exported constant, variable,
// function, type and method of the Go packages extensions and custom builds program against (the contract packages
// under ext/contracts), with their doc comments and Example functions, read with go/doc from the source. It reads like
// pkg.go.dev, on this site, so the site's search, llms.txt and the Agent Skills have it too.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const goRefRoot = "ext/contracts"

// goRefPackage is one package's page: its import path, name and rendered sections.
type goRefPackage struct {
	Dir, Import, Name, Synopsis string
	URL                         string // its page
	PocketBase                  bool   // one of PocketBase's own packages, not a contract
	Page                        string
}

// pbGoSkip are PocketBase folders whose Go code isn't an API anyone builds on (or is PocketOne's own, documented
// elsewhere).
var pbGoSkip = map[string]bool{"cmd": true, "migrations": true, "mails/templates": true, "plugins/jsvm": true, "ui": true,
	"ext": true, "site": true, "examples": true, "docs": true, "agent": true, "node_modules": true}

const pbGoURL = "/docs/reference/go/pocketbase"

// pbGoDirs lists PocketBase's own Go packages: the root package, core, apis, forms, mails, tools/..., plugins/... and
// tests (internal ones and testdata left out).
func pbGoDirs(root string) []string {
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		base := filepath.Base(path)
		if rel != "." && (pbGoSkip[rel] || base == "internal" || base == "testdata" || strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_")) {
			return filepath.SkipDir
		}
		if rel != "." {
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir // another module
			}
		}
		entries, _ := os.ReadDir(path)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
				dirs = append(dirs, rel)
				break
			}
		}
		return nil
	})
	sort.Strings(dirs)
	return dirs
}

// pbGoPackages reads and renders PocketBase's packages, at /docs/reference/go/pocketbase/<folder>.
func pbGoPackages(root string) []goRefPackage {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil
	}
	module := strings.TrimSpace(strings.TrimPrefix(strings.SplitN(string(raw), "\n", 2)[0], "module"))
	var out []goRefPackage
	for _, dir := range pbGoDirs(root) {
		imp, url := module, pbGoURL
		if dir != "." {
			imp, url = module+"/"+dir, pbGoURL+"/"+dir
		}
		if p, ok := goRefRead(root, dir, imp, url); ok {
			p.PocketBase = true
			out = append(out, p)
		}
	}
	return out
}

// goRefDirs lists the Go package folders under ext/contracts (internal ones and the cmd tools left out).
func goRefDirs(root string) []string {
	var dirs []string
	_ = filepath.WalkDir(filepath.Join(root, goRefRoot), func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		base := filepath.Base(path)
		if base == "internal" || base == "cmd" || base == "testdata" || strings.HasPrefix(base, ".") {
			return filepath.SkipDir
		}
		entries, _ := os.ReadDir(path)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
				dirs = append(dirs, filepath.ToSlash(rel))
				break
			}
		}
		return nil
	})
	sort.Strings(dirs)
	return dirs
}

// goRefURL is a package's page: /docs/reference/go/kv/v1 for ext/contracts/kv/v1, /docs/reference/go/contracts for
// ext/contracts itself.
func goRefURL(dir string) string {
	rel := strings.TrimPrefix(strings.TrimPrefix(dir, goRefRoot), "/")
	if rel == "" {
		rel = "contracts"
	}
	return "/docs/reference/go/" + rel
}

// goRefPackages reads and renders every package.
func goRefPackages(root string) []goRefPackage {
	var out []goRefPackage
	for _, dir := range goRefDirs(root) {
		if p, ok := goRefRead(root, dir, "pocketone.dev/"+dir, goRefURL(dir)); ok {
			out = append(out, p)
		}
	}
	return out
}

func goRefRead(root, dir, importPath, url string) (goRefPackage, bool) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		return goRefPackage{}, false
	}
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(root, dir, e.Name()), nil, parser.ParseComments)
		if err != nil {
			continue
		}
		// a _test.go file of another package (pkg_test) still holds the examples doc.NewFromFiles wants
		files = append(files, f)
	}
	pkg, err := doc.NewFromFiles(fset, files, importPath)
	if err != nil || pkg.Name == "" {
		return goRefPackage{}, false
	}
	gp := goRefPackage{Dir: dir, Import: importPath, Name: pkg.Name, Synopsis: pkg.Synopsis(pkg.Doc), URL: url}
	print := func(n any) string {
		var b bytes.Buffer
		_ = (&printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}).Fprint(&b, fset, n)
		return b.String()
	}
	md := func(text string) string {
		out := strings.TrimSpace(string(pkg.Markdown(strings.TrimSpace(text))))
		// go/doc links in-page symbols as #Get; the site's heading anchors are lowercase
		return goDocLinkRe.ReplaceAllStringFunc(out, func(l string) string { return "](#" + gid(l[3:len(l)-1]) + ")" })
	}
	example := func(ex *doc.Example) string {
		lines := strings.Split(print(ex.Code), "\n")
		if len(lines) >= 2 && strings.TrimSpace(lines[0]) == "{" && strings.TrimSpace(lines[len(lines)-1]) == "}" {
			lines = lines[1 : len(lines)-1]
		}
		code := dedent(strings.Join(lines, "\n"))
		if strings.TrimSpace(ex.Output) != "" {
			code += "\n// Output:\n// " + strings.ReplaceAll(strings.TrimSpace(ex.Output), "\n", "\n// ")
		}
		return code
	}
	examples := func(b *strings.Builder, exs []*doc.Example) {
		for _, ex := range exs {
			title := "Example"
			if ex.Suffix != "" {
				title += " (" + strings.ReplaceAll(ex.Suffix, "_", " ") + ")"
			}
			fmt.Fprintf(b, "%s:\n\n```go\n%s\n```\n\n", title, example(ex))
		}
	}
	funcEntry := func(b *strings.Builder, f *doc.Func, heading string) {
		decl := *f.Decl
		decl.Body, decl.Doc = nil, nil
		fmt.Fprintf(b, "### %s\n\n```go\n%s\n```\n\n", heading, print(&decl))
		if d := md(f.Doc); d != "" {
			b.WriteString(d + "\n\n")
		}
		examples(b, f.Examples)
	}
	values := func(b *strings.Builder, vs []*doc.Value) {
		for _, v := range vs {
			d := *v.Decl
			d.Doc = nil
			for _, n := range v.Names { // constants and variables have no heading; links to them land here
				fmt.Fprintf(b, "<a id=%q></a>", gid(n))
			}
			b.WriteString("\n\n")
			fmt.Fprintf(b, "```go\n%s\n```\n\n", print(&d))
			if t := md(v.Doc); t != "" {
				b.WriteString(t + "\n\n")
			}
		}
	}

	var b strings.Builder
	b.WriteString(refFrontMatter(pkg.Name+" (Go)", strings.TrimSpace(gp.Synopsis)))
	fmt.Fprintf(&b, "# %s\n\n", pkg.Name)
	fmt.Fprintf(&b, "```go\nimport %s %q\n```\n\n", pkg.Name, importPath)
	if d := md(pkg.Doc); d != "" {
		b.WriteString(d + "\n\n")
	}
	fmt.Fprintf(&b, "Source: [`%s`](%s/tree/main/%s).\n\n", path.Clean(dir), repoURL, dir)

	if len(pkg.Consts) > 0 {
		b.WriteString("## Constants\n\n")
		values(&b, pkg.Consts)
	}
	if len(pkg.Vars) > 0 {
		b.WriteString("## Variables\n\n")
		values(&b, pkg.Vars)
	}
	if len(pkg.Funcs) > 0 {
		b.WriteString("## Functions\n\n")
		for _, f := range pkg.Funcs {
			funcEntry(&b, f, f.Name)
		}
	}
	if len(pkg.Types) > 0 {
		b.WriteString("## Types\n\n")
		for _, t := range pkg.Types {
			d := *t.Decl
			d.Doc = nil
			fmt.Fprintf(&b, "### %s\n\n```go\n%s\n```\n\n", t.Name, print(&d))
			if doc := md(t.Doc); doc != "" {
				b.WriteString(doc + "\n\n")
			}
			examples(&b, t.Examples)
			values(&b, t.Consts)
			values(&b, t.Vars)
			for _, f := range t.Funcs {
				funcEntry(&b, f, f.Name)
			}
			for _, f := range t.Methods {
				funcEntry(&b, f, t.Name+"."+f.Name)
			}
		}
	}
	gp.Page = escapeEJS(b.String())
	return gp, true
}

// goRefIndex is /docs/reference/go: every package, by folder.
func goRefIndex(pkgs, pbpkgs []goRefPackage) string {
	var b strings.Builder
	b.WriteString(refFrontMatter("Go API", "The Go packages extensions and custom PocketOne builds program against: every contract's interface and types, and the helpers they share."))
	b.WriteString("# Go API\n\n")
	b.WriteString("The Go packages under [`ext/contracts`](" + repoURL + "/tree/main/ext/contracts): each [contract](/docs/contracts)'s " +
		"interface, the types its operations take and return, and the helpers it ships, for a custom build or an extension " +
		"that calls one. Generated from the source with `go/doc`, examples included. An extension's JavaScript side is in the " +
		"[JavaScript API](/docs/reference/javascript).\n\n")
	b.WriteString("## Contracts\n\n| Package | |\n|---|---|\n")
	for _, p := range pkgs {
		fmt.Fprintf(&b, "| [`%s`](%s) | %s |\n", strings.TrimPrefix(p.Import, "pocketone.dev/ext/contracts/"), p.URL, cell(strings.TrimSpace(p.Synopsis)))
	}
	b.WriteString("\n")
	if len(pbpkgs) > 0 {
		b.WriteString("## PocketBase\n\nPocketOne's core: the app (`core`), its HTTP API (`apis`), the tools it's built from, and the " +
			"test helpers (`tests`). A custom build or an extension imports these by the module path in each page's import line.\n\n")
		b.WriteString("| Package | |\n|---|---|\n")
		for _, p := range pbpkgs {
			name := strings.TrimPrefix(strings.TrimPrefix(p.URL, pbGoURL), "/")
			if name == "" {
				name = p.Name
			}
			fmt.Fprintf(&b, "| [`%s`](%s) | %s |\n", name, p.URL, cell(strings.TrimSpace(p.Synopsis)))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// goRefLink is the Go reference link for an @go target ("kvv1.Get", "kvv1.Store.Get", "kvv1.Entry"), "" when no
// package has it.
func goRefLink(pkgs []goRefPackage, target string) string {
	name, sym, ok := strings.Cut(target, ".")
	if !ok {
		return ""
	}
	for _, p := range pkgs {
		if p.Name == name && strings.Contains(p.Page, "\n### "+sym+"\n") {
			return p.URL + "#" + gid(sym)
		}
	}
	return ""
}

var goDocLinkRe = regexp.MustCompile(`\]\(#[A-Za-z_][\w.]*\)`)

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
		if min > 0 && len(l) >= min {
			lines[i] = l[min:]
		}
	}
	return strings.Join(lines, "\n")
}
