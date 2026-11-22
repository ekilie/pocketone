package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// convert turns one built docs page into its title and Markdown body. The article is the page's
// div.content[data-pagefind-body]; its title is the hidden data-pagefind-meta="title" span. pageURL makes in-page
// anchors and relative links absolute.
func convert(raw []byte, pageURL string) (title, body string, err error) {
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return "", "", err
	}
	article := find(doc, func(n *html.Node) bool { return n.DataAtom == atom.Div && hasAttr(n, "data-pagefind-body") })
	if article == nil {
		return "", "", fmt.Errorf("no article (div[data-pagefind-body])")
	}
	if t := find(article, func(n *html.Node) bool { return attr(n, "data-pagefind-meta") == "title" }); t != nil {
		title = strings.TrimSpace(text(t))
	}
	c := &conv{page: pageURL}
	c.blocks(article)
	return title, tidy(c.out.String()), nil
}

type conv struct {
	page string
	out  strings.Builder
	list []listState // open lists, innermost last
}

type listState struct {
	ordered bool
	n       int
}

// skip: parts of the article that aren't content.
func skip(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Nav, atom.Svg:
		return true
	}
	return hasClass(n, "toc") || hasClass(n, "hidden") && n.DataAtom == atom.Span || hasClass(n, "docs-footer")
}

// blocks writes n's children as Markdown blocks.
func (c *conv) blocks(n *html.Node) {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		c.block(ch)
	}
}

func (c *conv) para(s string) {
	s = strings.TrimSpace(s)
	if s != "" {
		c.out.WriteString("\n\n" + s + "\n\n")
	}
}

func (c *conv) block(n *html.Node) {
	if skip(n) {
		return
	}
	if n.Type == html.TextNode {
		if t := collapse(n.Data); strings.TrimSpace(t) != "" {
			c.para(t)
		}
		return
	}
	if n.Type != html.ElementNode {
		return
	}
	switch {
	case hasClass(n, "code-tabs") || hasClass(n, "tabs") && find(n, func(x *html.Node) bool { return hasClass(x, "tabs-content") }) != nil:
		c.tabs(n)
		return
	case hasClass(n, "code-wrapper"):
		c.code(n, "")
		return
	case hasClass(n, "accordion"):
		c.accordion(n)
		return
	case hasClass(n, "api-route"):
		// an API route: "GET" and the path, on one line
		method, path := "", ""
		if m := find(n, func(x *html.Node) bool { return x.DataAtom == atom.Strong }); m != nil {
			method = strings.TrimSpace(text(m))
		}
		if p := find(n, func(x *html.Node) bool { return hasClass(x, "content") }); p != nil {
			path = strings.TrimSpace(collapse(text(p)))
		}
		c.para("`" + strings.TrimSpace(method+" "+path) + "`")
		return
	case hasClass(n, "section-title"):
		c.para("**" + strings.TrimSpace(collapse(text(n))) + "**")
		return
	}
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		level := map[atom.Atom]int{atom.H1: 2, atom.H2: 2, atom.H3: 2, atom.H4: 3, atom.H5: 4, atom.H6: 5}[n.DataAtom]
		// plain text: the site's headings link to themselves
		c.para(strings.Repeat("#", level) + " " + strings.TrimSpace(collapse(text(n))))
	case atom.P:
		c.para(c.inline(n))
	case atom.Ul, atom.Ol:
		c.listBlock(n, n.DataAtom == atom.Ol)
	case atom.Table:
		c.table(n)
	case atom.Blockquote:
		var sub conv
		sub.page = c.page
		sub.blocks(n)
		lines := strings.Split(strings.TrimSpace(tidy(sub.out.String())), "\n")
		for i, l := range lines {
			lines[i] = strings.TrimRight("> "+l, " ")
		}
		c.para(strings.Join(lines, "\n"))
	case atom.Pre:
		c.code(n, "")
	case atom.Hr:
		c.para("---")
	case atom.Img:
		c.para(c.image(n))
	case atom.Br:
		c.out.WriteString("\n")
	case atom.Div, atom.Section, atom.Article, atom.Main, atom.Span:
		// a container: its children may be blocks or inline text (an alert box, a wrapper)
		if hasBlockChild(n) {
			c.blocks(n)
		} else {
			c.para(c.inline(n))
		}
	case atom.Button:
		// a stray button's label (tab and accordion headers are handled with their parts)
	default:
		c.para(c.inline(n))
	}
}

// hasBlockChild reports whether n holds block elements (so it isn't one paragraph of inline text).
func hasBlockChild(n *html.Node) bool {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type != html.ElementNode || skip(ch) {
			continue
		}
		switch ch.DataAtom {
		case atom.P, atom.Div, atom.Ul, atom.Ol, atom.Table, atom.Pre, atom.Blockquote, atom.H1, atom.H2, atom.H3,
			atom.H4, atom.H5, atom.H6, atom.Hr, atom.Section, atom.Img:
			return true
		}
	}
	return false
}

// tabs writes each tab as a labelled part: code tabs (Go / JavaScript) as labelled code blocks, other tabs as a bold
// label before their content.
func (c *conv) tabs(n *html.Node) {
	header := find(n, func(x *html.Node) bool { return hasClass(x, "tabs-header") })
	content := find(n, func(x *html.Node) bool { return hasClass(x, "tabs-content") })
	var labels []string
	if header != nil {
		for b := header.FirstChild; b != nil; b = b.NextSibling {
			if b.Type == html.ElementNode && b.DataAtom == atom.Button {
				labels = append(labels, strings.TrimSpace(collapse(text(b))))
			}
		}
	}
	i := 0
	for t := content.FirstChild; t != nil; t = t.NextSibling {
		if t.Type != html.ElementNode || !hasClass(t, "tab-item") {
			continue
		}
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		i++
		if code := find(t, func(x *html.Node) bool { return hasClass(x, "code-wrapper") }); code != nil && onlyCode(t) {
			c.code(code, label)
			continue
		}
		if label != "" {
			c.para("**" + label + "**")
		}
		c.blocks(t)
	}
}

// onlyCode reports whether a tab holds just a code block.
func onlyCode(t *html.Node) bool {
	n := 0
	for ch := t.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode {
			n++
		} else if ch.Type == html.TextNode && strings.TrimSpace(ch.Data) != "" {
			return false
		}
	}
	return n == 1
}

var langs = map[string]string{"go": "go", "javascript": "js", "js": "js", "dart": "dart", "typescript": "ts", "sql": "sql",
	"bash": "bash", "shell": "bash", "html": "html", "json": "json"}

// code writes a code block; label (a tab's) names the language when it's one, else a language-* class does.
func (c *conv) code(n *html.Node, label string) {
	lines := strings.Split(strings.Trim(textSkipping(n, func(x *html.Node) bool { return x.DataAtom == atom.Button }), "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	src := strings.Join(lines, "\n")
	lang := langs[strings.ToLower(label)]
	if label == "" {
		if code := find(n, func(x *html.Node) bool { return x.DataAtom == atom.Code }); code != nil {
			for _, cl := range strings.Fields(attr(code, "class")) {
				if strings.HasPrefix(cl, "language-") {
					lang = strings.TrimPrefix(cl, "language-")
				}
			}
		}
	}
	fence := "```"
	for strings.Contains(src, fence) {
		fence += "`"
	}
	block := fence + lang + "\n" + src + "\n" + fence
	if label != "" && lang == "" {
		block = "**" + label + "**\n\n" + block
	} else if label != "" {
		block = label + ":\n\n" + block
	}
	c.para(block)
}

// accordion writes its header as a heading, then its (hidden on the site) content.
func (c *conv) accordion(n *html.Node) {
	if h := find(n, func(x *html.Node) bool { return hasClass(x, "accordion-header") }); h != nil {
		c.para("### " + strings.TrimSpace(collapse(text(h))))
	}
	if body := find(n, func(x *html.Node) bool { return hasClass(x, "accordion-content") }); body != nil {
		c.blocks(body)
	}
}

func (c *conv) listBlock(n *html.Node, ordered bool) {
	c.list = append(c.list, listState{ordered: ordered})
	depth := len(c.list) - 1
	var b strings.Builder
	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.DataAtom != atom.Li {
			continue
		}
		st := &c.list[depth]
		st.n++
		marker := "- "
		if ordered {
			marker = fmt.Sprintf("%d. ", st.n)
		}
		var sub conv
		sub.page, sub.list = c.page, c.list
		hasNested := false
		var inline strings.Builder
		for ch := li.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type == html.ElementNode && (ch.DataAtom == atom.Ul || ch.DataAtom == atom.Ol || hasClass(ch, "code-wrapper") || ch.DataAtom == atom.P || ch.DataAtom == atom.Pre || ch.DataAtom == atom.Div && hasBlockChild(ch)) {
				hasNested = true
				if s := strings.TrimSpace(inline.String()); s != "" {
					sub.para(s)
					inline.Reset()
				}
				sub.block(ch)
				continue
			}
			inline.WriteString(c.inlineNode(ch))
		}
		if s := strings.TrimSpace(collapse(inline.String())); s != "" {
			sub.para(s)
		}
		item := strings.TrimSpace(tidy(sub.out.String()))
		indent := strings.Repeat(" ", len(marker))
		lines := strings.Split(item, "\n")
		for i := range lines {
			if i > 0 && lines[i] != "" {
				lines[i] = indent + lines[i]
			}
		}
		_ = hasNested
		b.WriteString(marker + strings.Join(lines, "\n") + "\n")
	}
	c.list = c.list[:depth]
	c.para(b.String())
}

func (c *conv) table(n *html.Node) {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type != html.ElementNode {
				continue
			}
			if ch.DataAtom == atom.Tr {
				var row []string
				for td := ch.FirstChild; td != nil; td = td.NextSibling {
					if td.Type == html.ElementNode && (td.DataAtom == atom.Td || td.DataAtom == atom.Th) {
						cell := strings.TrimSpace(collapse(c.cellText(td)))
						row = append(row, strings.ReplaceAll(cell, "|", `\|`))
					}
				}
				rows = append(rows, row)
				continue
			}
			walk(ch)
		}
	}
	walk(n)
	if len(rows) == 0 {
		return
	}
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	var b strings.Builder
	for i, r := range rows {
		for len(r) < cols {
			r = append(r, "")
		}
		b.WriteString("| " + strings.Join(r, " | ") + " |\n")
		if i == 0 {
			b.WriteString("|" + strings.Repeat("---|", cols) + "\n")
		}
	}
	c.para(b.String())
}

// cellText is a table cell's content on one line: inline Markdown, code blocks as inline code.
func (c *conv) cellText(td *html.Node) string {
	var b strings.Builder
	for ch := td.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode && (hasClass(ch, "code-wrapper") || ch.DataAtom == atom.Pre) {
			b.WriteString(" `" + strings.Join(strings.Fields(text(ch)), " ") + "` ")
			continue
		}
		if ch.Type == html.ElementNode && (ch.DataAtom == atom.P || ch.DataAtom == atom.Div || ch.DataAtom == atom.Br) {
			b.WriteString(" " + c.inline(ch) + " ")
			continue
		}
		b.WriteString(c.inlineNode(ch))
	}
	return b.String()
}

// inline is n's children as one line of inline Markdown.
func (c *conv) inline(n *html.Node) string {
	var b strings.Builder
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		b.WriteString(c.inlineNode(ch))
	}
	return collapse(b.String())
}

func (c *conv) inlineNode(n *html.Node) string {
	if skip(n) {
		return ""
	}
	switch n.Type {
	case html.TextNode:
		return collapse(n.Data)
	case html.ElementNode:
	default:
		return ""
	}
	switch n.DataAtom {
	case atom.Strong, atom.B:
		if s := strings.TrimSpace(c.inline(n)); s != "" {
			return wrapSpaces(c.inline(n), "**")
		}
		return ""
	case atom.Em, atom.I:
		if s := strings.TrimSpace(c.inline(n)); s != "" {
			return wrapSpaces(c.inline(n), "*")
		}
		return ""
	case atom.Code:
		t := text(n)
		if strings.Contains(t, "\n") {
			return " `" + strings.Join(strings.Fields(t), " ") + "` "
		}
		tick := "`"
		if strings.Contains(t, "`") {
			tick = "`` "
			return tick + t + " ``"
		}
		return tick + t + tick
	case atom.A:
		label := strings.TrimSpace(c.inline(n))
		href := c.abs(attr(n, "href"))
		if label == "" || href == "" {
			return label
		}
		return "[" + label + "](" + href + ")"
	case atom.Img:
		return c.image(n)
	case atom.Br:
		return " "
	case atom.Button:
		return ""
	}
	return c.inline(n)
}

func (c *conv) image(n *html.Node) string {
	src := c.abs(attr(n, "src"))
	if src == "" {
		return ""
	}
	return "![" + attr(n, "alt") + "](" + src + ")"
}

// abs makes a link absolute: /docs/x → https://pocketbase.io/docs/x, #x → this page's URL #x.
func (c *conv) abs(href string) string {
	switch {
	case href == "":
		return ""
	case strings.HasPrefix(href, "//"):
		return "https:" + href
	case strings.HasPrefix(href, "/"):
		return site + href
	case strings.Contains(href, "://") || strings.HasPrefix(href, "mailto:"):
		return href
	}
	return resolve(c.page, href) // #anchor, page.html, ../dir/page.html
}

// wrapSpaces puts marks around s, keeping its outer spaces outside them (" a " → " **a** ").
func wrapSpaces(s, mark string) string {
	t := strings.TrimSpace(s)
	lead := s[:len(s)-len(strings.TrimLeft(s, " "))]
	trail := s[len(strings.TrimRight(s, " ")):]
	return lead + mark + t + mark + trail
}

var spaceRe = regexp.MustCompile(`[ \t\r\n]+`)

func collapse(s string) string { return spaceRe.ReplaceAllString(s, " ") }

var blankRe = regexp.MustCompile(`\n{3,}`)

// tidy trims trailing spaces and squeezes blank lines (never inside code blocks, which keep theirs).
func tidy(s string) string {
	lines := strings.Split(s, "\n")
	inFence := ""
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") {
			if inFence == "" {
				inFence = strings.TrimRight(t, "abcdefghijklmnopqrstuvwxyz")
			} else if strings.TrimSpace(t) == inFence {
				inFence = ""
			}
			continue
		}
		if inFence == "" {
			lines[i] = strings.TrimRight(l, " \t")
		}
	}
	return strings.TrimSpace(blankRe.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

// text is n's text, as written (code keeps its line breaks).
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
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

func find(n *html.Node, match func(*html.Node) bool) *html.Node {
	if n.Type == html.ElementNode && match(n) {
		return n
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if f := find(ch, match); f != nil {
			return f
		}
	}
	return nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

func hasClass(n *html.Node, class string) bool {
	if n.Type != html.ElementNode {
		return false
	}
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}
