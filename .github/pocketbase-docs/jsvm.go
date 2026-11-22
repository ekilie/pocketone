package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// The JSVM reference (pocketbase.io/jsvm): typedoc's pages for PocketBase's JS API, generated from the site's
// jsvm/types.d.ts and committed in the site repo as static/jsvm/**/*.html. Each page becomes jsvm/<same path>.md
// under outDir, starting with its pocketbase.io/jsvm URL; its links point there.

const jsvmSite = site + "/jsvm/"

// convertJSVM converts every page of the reference under dir (the site's static/jsvm).
func convertJSVM(dir string) ([]page, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".html") {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no pages under %s", dir)
	}
	sort.Strings(files)
	var out []page
	for _, f := range files {
		rel, _ := filepath.Rel(dir, f)
		rel = filepath.ToSlash(rel)
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		title, body, err := convertTypedoc(raw, jsvmSite+rel)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		out = append(out, page{slug: strings.TrimSuffix(rel, ".html"), title: title, source: jsvmSite + rel, body: body})
	}
	return out, nil
}

// convertTypedoc turns one typedoc page into its title and Markdown body.
func convertTypedoc(raw []byte, pageURL string) (title, body string, err error) {
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return "", "", err
	}
	content := find(doc, func(n *html.Node) bool { return hasClass(n, "col-content") })
	if content == nil {
		return "", "", fmt.Errorf("no div.col-content")
	}
	if h := find(content, func(n *html.Node) bool { return n.DataAtom == atom.H1 }); h != nil {
		title = strings.TrimSpace(collapse(text(h)))
	}
	t := &typedoc{conv: conv{page: pageURL}}
	t.blocks(content)
	return title, tidy(t.out.String()), nil
}

// typedoc converts typedoc's markup: signatures as code, parameter lists as lists of name: type, member sections
// under their names; the breadcrumb, the title, the index panel and the "Defined in" lines are left out.
type typedoc struct {
	conv
}

func (t *typedoc) blocks(n *html.Node) {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		t.block(ch)
	}
}

func (t *typedoc) block(n *html.Node) {
	if n.Type != html.ElementNode {
		if n.Type == html.TextNode && strings.TrimSpace(n.Data) != "" {
			t.para(collapse(n.Data))
		}
		return
	}
	switch {
	case n.DataAtom == atom.Script || n.DataAtom == atom.Style || n.DataAtom == atom.Svg || n.DataAtom == atom.Button:
		return
	case hasClass(n, "tsd-page-title"), hasClass(n, "tsd-index-panel"), hasClass(n, "tsd-sources"),
		hasClass(n, "tsd-anchor-icon"), hasClass(n, "tsd-anchor"), hasClass(n, "tsd-breadcrumb"):
		return
	case hasClass(n, "tsd-signatures") || hasClass(n, "tsd-description") || hasClass(n, "tsd-parameters") ||
		hasClass(n, "tsd-panel") || hasClass(n, "tsd-member-group") || hasClass(n, "tsd-panel-group"):
		t.blocks(n) // typedoc's containers (some are lists): their parts, not a list
		return
	case hasClass(n, "tsd-signature"):
		t.para("```ts\n" + signature(n) + "\n```")
		return
	case hasClass(n, "tsd-parameter-list"):
		t.params(n)
		return
	case hasClass(n, "tsd-comment"):
		// a doc comment: the docs pages' converter handles its paragraphs, lists and code
		c := conv{page: t.page}
		c.blocks(n)
		t.para(tidy(c.out.String()))
		return
	case n.DataAtom == atom.Pre:
		t.pre(n)
		return
	}
	switch n.DataAtom {
	case atom.H1:
		return // the title
	case atom.H2:
		t.para("## " + heading(n))
	case atom.H3:
		t.para("### " + heading(n))
	case atom.H4, atom.H5:
		t.para("**" + heading(n) + "**")
	case atom.P, atom.Ul, atom.Ol, atom.Table, atom.Blockquote, atom.Hr, atom.Img:
		c := conv{page: t.page}
		c.block(n)
		t.para(tidy(c.out.String()))
	case atom.A:
		t.para(t.inline(n))
	default:
		// containers: sections, member groups, signature lists, descriptions, details
		t.blocks(n)
	}
}

// pre is a code example, with its language from the code element's class (language-js).
func (t *typedoc) pre(n *html.Node) {
	lang := ""
	if code := find(n, func(x *html.Node) bool { return x.DataAtom == atom.Code }); code != nil {
		for _, c := range strings.Fields(attr(code, "class")) {
			if strings.HasPrefix(c, "language-") {
				lang = strings.TrimPrefix(c, "language-")
			}
		}
	}
	lines := strings.Split(strings.Trim(textNoButtons(n), "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	t.para("```" + lang + "\n" + strings.Join(lines, "\n") + "\n```")
}

// params writes a parameter list: "- `name: type`", then its description.
func (t *typedoc) params(n *html.Node) {
	var b strings.Builder
	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.DataAtom != atom.Li {
			continue
		}
		head := ""
		if h := find(li, func(x *html.Node) bool { return x.DataAtom == atom.H5 }); h != nil {
			head = signature(h)
		}
		desc := ""
		if c := find(li, func(x *html.Node) bool { return hasClass(x, "tsd-comment") }); c != nil {
			cc := conv{page: t.page}
			cc.blocks(c)
			desc = strings.Join(strings.Fields(tidy(cc.out.String())), " ")
		}
		line := "- `" + head + "`"
		if desc != "" {
			line += ": " + desc
		}
		b.WriteString(line + "\n")
	}
	t.para(b.String())
}

// signature is a signature's text on one line (typedoc breaks names with <wbr>, which text drops).
func signature(n *html.Node) string {
	return strings.TrimSpace(collapse(textSkipping(n, func(x *html.Node) bool {
		return hasClass(x, "tsd-anchor-icon") || x.DataAtom == atom.Svg
	})))
}

func heading(n *html.Node) string { return signature(n) }

func textNoButtons(n *html.Node) string {
	return textSkipping(n, func(x *html.Node) bool { return x.DataAtom == atom.Button })
}

// textSkipping is n's text without the subtrees skip matches.
func textSkipping(n *html.Node, skip func(*html.Node) bool) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.ElementNode && skip(x) {
			return
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		if x.Type == html.ElementNode && x.DataAtom == atom.Br {
			b.WriteString("\n")
		}
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return b.String()
}

// resolve makes href absolute against base, the way a browser does (../interfaces/core.Record.html).
func resolve(base, href string) string {
	b, err := url.Parse(base)
	if err != nil {
		return href
	}
	r, err := url.Parse(href)
	if err != nil {
		return href
	}
	return b.ResolveReference(r).String()
}

// writeJSVM writes the reference under outDir/jsvm, with an index README, and removes pages the site no longer has.
func writeJSVM(pages []page, commit string) error {
	root := filepath.Join(outDir, "jsvm")
	want := map[string]bool{}
	for _, p := range pages {
		md := fmt.Sprintf("Source: %s\n\n# %s\n\n%s\n", p.source, p.title, strings.TrimSpace(p.body))
		rel := p.slug + ".md"
		want[rel] = true
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := writeIfChanged(full, md); err != nil {
			return err
		}
	}
	want["README.md"] = true
	want["types.d.ts"] = true // written by run
	if err := writeIfChanged(filepath.Join(root, "README.md"), jsvmReadme(pages, commit)); err != nil {
		return err
	}
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if !want[filepath.ToSlash(rel)] {
			logf("removing jsvm/%s", filepath.ToSlash(rel))
			return os.Remove(p)
		}
		return nil
	})
}

func jsvmReadme(pages []page, commit string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `# PocketBase JSVM reference

PocketBase's JS API reference, [%[1]s](%[1]s), converted to Markdown: every function, variable, module, class and
interface the JS hooks see (`+"`$app`, `routerAdd`, `$http`, the `core` types"+`...), with their descriptions and
examples. Typedoc generates the site from the PocketBase site's `+"`jsvm/types.d.ts`"+`; these files come from its
pages at commit `+"`%[2]s`"+`. Each page starts with its pocketbase.io URL. Don't edit them: run
`+"`.github/pocketbase-docs`"+` again.

[types.d.ts](types.d.ts) is that declarations file itself, every declaration with its doc comment in one place (about
800 KB): handy to search. It isn't published on pocketbase.io; its source is
[jsvm/types.d.ts in the mirror](https://github.com/pocketonehq/pocketbase-site/blob/%[3]s/jsvm/types.d.ts) at the same
commit.

PocketOne runs pb_hooks on its own engine (QuickJS), with the same API plus its own (`+"`.async`"+` on every Go
function, imports, the extensions' globals such as `+"`$llm`"+`). PocketOne's full declarations are in each app's
`+"`pb_data/types.d.ts`"+`.

`, jsvmSite, short(commit), commit)
	groups := map[string][]page{}
	for _, p := range pages {
		dir := path.Dir(p.slug)
		if dir == "." {
			dir = ""
		}
		groups[dir] = append(groups[dir], p)
	}
	names := map[string]string{"functions": "Functions", "variables": "Variables", "modules": "Modules", "classes": "Classes",
		"interfaces": "Interfaces", "types": "Types", "enums": "Enums", "": "Overview"}
	var dirs []string
	for d := range groups {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, d := range dirs {
		name := names[d]
		if name == "" {
			name = d
		}
		fmt.Fprintf(&b, "## %s\n\n", name)
		for _, p := range groups[d] {
			fmt.Fprintf(&b, "- [%s](%s.md)\n", strings.TrimPrefix(p.title, ""), p.slug)
		}
		b.WriteString("\n")
	}
	b.WriteString("## License\n\nThe PocketBase site and this reference are MIT licensed, Copyright (c) Gani Georgiev.\n")
	return b.String()
}
