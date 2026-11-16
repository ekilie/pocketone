package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// searchOut is the index the site's search box (assets/search.js) loads: one entry per page section.
const searchOut = "site/pb_pages/assets/search-index.json"

// searchRoots are the site folders whose pages are searchable.
var searchRoots = []string{"site/pb_pages/(main)/docs", "site/pb_pages/(main)/blog"}

type searchEntry struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	Title   string `json:"title"`   // page title
	Heading string `json:"heading"` // section heading; empty for the part before the first H2
	Text    string `json:"text"`
}

// searchIndex builds the index from every .md and .ejs page under searchRoots, reading generated pages from want (so
// it never lags behind them) and the rest from disk. Pages split into sections at H2-H4 headings; each section's
// URL has the anchor goldmark gives that heading.
func searchIndex(root string, want map[string]string) (string, error) {
	pages := map[string]string{} // repo path -> content
	for _, dir := range searchRoots {
		err := filepath.WalkDir(filepath.Join(root, dir), func(full string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, full)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if d.Name() == "_private" {
					return filepath.SkipDir
				}
				return nil
			}
			n := d.Name()
			if strings.HasPrefix(n, "+") || n == "index.ejs" || !(strings.HasSuffix(n, ".md") || strings.HasSuffix(n, ".ejs")) {
				return nil
			}
			raw, err := os.ReadFile(full)
			if err != nil {
				return err
			}
			content := string(raw)
			if strings.HasSuffix(n, ".ejs") {
				content = inlineIncludes(root, path.Dir(rel), content)
			}
			pages[rel] = content
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	for p, content := range want { // generated pages, including ones not written yet
		if strings.HasSuffix(p, ".md") && (strings.HasPrefix(p, searchRoots[0]+"/") || strings.HasPrefix(p, searchRoots[1]+"/")) {
			pages[p] = content
		}
	}

	paths := make([]string, 0, len(pages))
	for p := range pages {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var entries []searchEntry
	for _, p := range paths {
		for _, e := range pageSections(p, pages[p]) {
			e.ID = len(entries)
			entries = append(entries, e)
		}
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return string(raw) + "\n", nil
}

// pageURL maps a site file to its URL: route groups like (main) drop out, and index pages take their folder's URL.
func pageURL(p string) string {
	p = strings.TrimPrefix(p, "site/pb_pages")
	var parts []string
	for _, s := range strings.Split(p, "/") {
		if s == "" || (strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")) {
			continue
		}
		parts = append(parts, s)
	}
	last := parts[len(parts)-1]
	last = strings.TrimSuffix(strings.TrimSuffix(last, ".md"), ".ejs")
	if last == "index" {
		parts = parts[:len(parts)-1]
	} else {
		parts[len(parts)-1] = last
	}
	return "/" + strings.Join(parts, "/")
}

var (
	frontRe    = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n`)
	titleRe    = regexp.MustCompile(`(?m)^title:\s*(.+?)\s*$`)
	commentRe  = regexp.MustCompile(`(?s)<!--.*?-->`)
	ejsRe      = regexp.MustCompile(`(?s)<%.*?%>`)
	scriptRe   = regexp.MustCompile(`(?is)<(script|style)\b.*?</(script|style)>`)
	htmlHRe    = regexp.MustCompile(`(?is)<h([1-3])\b[^>]*>(.*?)</h[1-3]>`)
	tagRe      = regexp.MustCompile(`(?s)<[^>]+>`)
	imageRe    = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	linkRe     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	refLinkRe  = regexp.MustCompile(`(?m)^\s*\[[^\]]+\]:\s*\S+.*$`)
	headingRe  = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)
	includeRe  = regexp.MustCompile(`<%-\s*include\(\s*['"]([^'"]+)['"]\s*\)\s*%>`)
	bulletRe   = regexp.MustCompile(`(?m)^\s*(?:[-+*]|\d+\.)\s+`)
	spaceRe    = regexp.MustCompile(`\s+`)
	markRe     = regexp.MustCompile("[`*|>]+")
	entityRepl = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&nbsp;", " ")
)

// pageSections splits one page into searchable sections.
func pageSections(p, content string) []searchEntry {
	url := pageURL(p)
	content = strings.ReplaceAll(content, "\r\n", "\n")
	title := ""
	if m := frontRe.FindStringSubmatch(content); m != nil {
		if t := titleRe.FindStringSubmatch(m[1]); t != nil {
			title = unquote(t[1])
		}
		content = content[len(m[0]):]
	}
	content = strings.ReplaceAll(content, "<%%", "<%")
	content = commentRe.ReplaceAllString(content, "")
	if strings.HasSuffix(p, ".ejs") {
		// An EJS page: drop code and tags, turning <h1>-<h3> into Markdown headings so it splits the same way.
		content = ejsRe.ReplaceAllString(content, "")
		content = scriptRe.ReplaceAllString(content, "")
		content = htmlHRe.ReplaceAllStringFunc(content, func(s string) string {
			m := htmlHRe.FindStringSubmatch(s)
			return "\n" + strings.Repeat("#", int(m[1][0]-'0')) + " " + spaceRe.ReplaceAllString(tagRe.ReplaceAllString(m[2], ""), " ") + "\n"
		})
		content = tagRe.ReplaceAllString(content, " ")
		content = entityRepl.Replace(content)
	}

	ids := map[string]bool{}
	var out []searchEntry
	cur := searchEntry{URL: url}
	var text []string
	flush := func() {
		t := cleanText(strings.Join(text, "\n"))
		if t != "" || cur.Heading != "" {
			out = append(out, searchEntry{URL: cur.URL, Title: title, Heading: cur.Heading, Text: t})
		}
		text = nil
	}
	fence := false
	for _, l := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = !fence
			continue
		}
		if !fence {
			if m := headingRe.FindStringSubmatch(l); m != nil {
				h := headingText(m[2])
				id := headingID(h, ids)
				if len(m[1]) == 1 {
					if title == "" {
						title = h
					}
					continue
				}
				if len(m[1]) > 4 {
					text = append(text, h)
					continue
				}
				flush()
				cur = searchEntry{URL: url + "#" + id, Heading: h}
				continue
			}
		}
		text = append(text, l)
	}
	flush()
	if title == "" {
		title = url
	}
	for i := range out {
		out[i].Title = title
	}
	return out
}

func unquote(s string) string {
	var v string
	if strings.HasPrefix(s, `"`) && json.Unmarshal([]byte(s), &v) == nil {
		return v
	}
	return strings.Trim(s, `'"`)
}

// headingText is a heading's plain text, as goldmark sees it for the heading ID.
func headingText(s string) string {
	s = imageRe.ReplaceAllString(s, "$1")
	s = linkRe.ReplaceAllString(s, "$1")
	s = tagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(strings.NewReplacer("`", "", "**", "", "__", "").Replace(s))
}

// headingID repeats goldmark's automatic heading ID (parser.WithAutoHeadingID): ASCII letters, digits and '_', lowercased;
// a run of spaces and '-' becomes one '-'; everything else (including non-ASCII) drops; repeats on a page get -1, -2, ...
func headingID(text string, seen map[string]bool) string {
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c >= 0x80:
		case c >= 'A' && c <= 'Z':
			b.WriteByte(c + 'a' - 'A')
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '_':
			b.WriteByte(c)
		case c == ' ' || c == '\t' || c == '-':
			if s := b.String(); !strings.HasSuffix(s, "-") {
				b.WriteByte('-')
			}
		}
	}
	id := b.String()
	if id == "" {
		id = "heading"
	}
	if !seen[id] {
		seen[id] = true
		return id
	}
	for i := 1; ; i++ {
		n := fmt.Sprintf("%s-%d", id, i)
		if !seen[n] {
			seen[n] = true
			return n
		}
	}
}

// cleanText turns a section's Markdown into plain text for the index. Code stays: commands and config keys are
// what people search for.
func cleanText(s string) string {
	s = dropTableHeads(s)
	s = refLinkRe.ReplaceAllString(s, "")
	s = bulletRe.ReplaceAllString(s, "")
	s = imageRe.ReplaceAllString(s, "$1")
	s = linkRe.ReplaceAllString(s, "$1")
	s = tagRe.ReplaceAllString(s, " ")
	s = markRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "---", " ")
	s = entityRepl.Replace(s)
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

// inlineIncludes replaces an EJS page's include('name') of a Markdown partial with the partial (its front matter
// dropped, but its title kept if the page has none), looking in _private folders from the page's folder up, as
// PocketPages does.
func inlineIncludes(root, dir, content string) string {
	return includeRe.ReplaceAllStringFunc(content, func(s string) string {
		name := includeRe.FindStringSubmatch(s)[1]
		if !strings.HasSuffix(name, ".md") {
			name += ".md"
		}
		for d := dir; strings.HasPrefix(d, "site/pb_pages"); d = path.Dir(d) {
			raw, err := os.ReadFile(filepath.Join(root, d, "_private", name))
			if err == nil {
				return string(raw)
			}
		}
		return ""
	})
}

var tableSepRe = regexp.MustCompile(`^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$`)

// dropTableHeads removes each Markdown table's header and separator rows ("| Key | Type | ... |"), which would
// otherwise lead many snippets with the same column names.
func dropTableHeads(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, l := range lines {
		if tableSepRe.MatchString(l) {
			if n := len(out); n > 0 && strings.Contains(out[n-1], "|") {
				out = out[:n-1]
			}
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}
