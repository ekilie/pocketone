package main

// Agent-facing copies of the docs, written from the same pages the sites serve:
//
//   - llms.txt and llms-full.txt at each site's root (https://llmstxt.org): an index of every docs page in nav order,
//     and every page's Markdown in one file.
//   - The Agent Skills' reference folders (agent/skills/<skill>/reference/docs/...): one plain-Markdown file per docs
//     page, so a skill works offline and never lags behind the site.
//
// The skills' SKILL.md files are hand-written; everything under their reference/ folders is generated.

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type docsSite struct {
	Name, URL, Summary, Intro string
	Pages                     string            // the site's pb_pages folder
	Toc                       string            // its docs nav (Markdown: "# Section" headings and "- [Title](/docs/...)" links)
	SkillRef                  string            // the skill's reference folder
	Extra                     map[string]string // more reference files: reference path -> repo path
	Within                    string            // a part of the site (a URL prefix): only its pages, and no llms.txt of its own
	Pick                      []string          // only these pages (a URL, or a part of the site), at their usual paths, and no llms.txt
	Skip                      []string          // parts of the site (URL prefixes) left out of the skill's reference (llms.txt keeps them)
}

// under reports whether URL u is p or a page below it.
func under(u, p string) bool { return u == p || strings.HasPrefix(u, p+"/") }

func underAny(u string, ps []string) bool {
	for _, p := range ps {
		if under(u, p) {
			return true
		}
	}
	return false
}

var docsSites = []docsSite{
	{
		Name:    "PocketOne",
		URL:     "https://pocketone.dev",
		Summary: "PocketOne is a community fork of PocketBase (one Go binary: database, auth, files, realtime, admin UI) with extensions for point-in-time recovery, backups, mail, many instances on one server, deploys, tokens and OAuth, MCP, and PocketPages server-rendered sites.",
		Intro: "PocketOne takes the same commands and flags as PocketBase, plus its own; core PocketBase topics live at https://pocketbase.io/docs. " +
			"Agent Skills for PocketOne and PocketPages: https://github.com/pocketonehq/pocketone/tree/main/agent (Claude Code: `/plugin marketplace add pocketonehq/pocketone`).",
		Pages:    "site/pb_pages",
		Toc:      "site/pb_pages/(main)/_private/toc.md",
		SkillRef: "agent/skills/pocketone/reference",
		// each in its own skill
		Skip: []string{"/docs/extensions/pages", "/docs/writing-an-extension"},
	},
	{
		// writing an extension: the guide, the contracts, and the catalogue of extensions that exist
		Name:     "PocketOne extensions",
		URL:      "https://pocketone.dev",
		Pages:    "site/pb_pages",
		Pick:     []string{"/docs/writing-an-extension", "/docs/contracts", "/docs/extensions", "/docs/config-reference"},
		Skip:     []string{"/docs/extensions/pages"},
		SkillRef: "agent/skills/pocketone-extensions/reference",
	},
	{
		// PocketPages' docs are the pages extension's docs (ext/pages/docs), part of pocketone.dev
		Name:     "PocketPages",
		URL:      "https://pocketone.dev",
		Pages:    "site/pb_pages",
		Within:   "/docs/extensions/pages",
		SkillRef: "agent/skills/pocketpages/reference",
	},
}

// agentOutDirs are the generated folders that sync also prunes (a removed page drops its copy).
func agentOutDirs() []string {
	var out []string
	for _, s := range docsSites {
		out = append(out, s.SkillRef)
	}
	return out
}

// agentFiles adds every site's llms.txt, llms-full.txt and skill reference files to out. Generated pages are read
// from out (so they never lag behind), the rest from disk.
func agentFiles(root string, out map[string]string) error {
	for _, s := range docsSites {
		pages, err := docsPages(root, s, out)
		if err != nil {
			return err
		}
		if s.Within != "" {
			// the skill gets only this part of the site, at paths relative to it (the landing page is docs/index.md)
			sub := map[string]docsPage{}
			for u, p := range pages {
				if under(u, s.Within) {
					rel := strings.TrimPrefix(strings.TrimPrefix(u, s.Within), "/")
					if rel == "" {
						rel = "index"
					} else if strings.HasSuffix(p.file, "/index.md") {
						rel += "/index"
					}
					p.file = rel + ".md"
					sub[u] = p
				}
			}
			for _, p := range sub {
				out[s.SkillRef+"/docs/"+p.file] = p.body
			}
			continue
		}
		if s.Pick != nil {
			for u, p := range pages {
				if underAny(u, s.Pick) && !underAny(u, s.Skip) {
					out[s.SkillRef+"/docs/"+p.file] = p.body
				}
			}
			continue
		}
		toc := out[s.Toc] // the nav this run generated, so llms.txt follows it in one run
		if toc == "" {
			raw, err := os.ReadFile(filepath.Join(root, s.Toc))
			if err != nil {
				return err
			}
			toc = string(raw)
		}
		sections := parseToc(toc)
		if s.Within == "" {
			// extension sites' own navs (toc-<name>.md), each section titled "<extension>: <section>"
			var navFiles []string
			for p := range out {
				if path.Dir(p) == path.Dir(s.Toc) && strings.HasPrefix(path.Base(p), "toc-") {
					navFiles = append(navFiles, p)
				}
			}
			sort.Strings(navFiles)
			for _, f := range navFiles {
				secs := parseToc(out[f])
				for i := range secs {
					if i > 0 {
						secs[i].title = secs[0].title + ": " + secs[i].title
					}
				}
				sections = append(sections, secs...)
			}
		}

		// Reference files: every docs page, at its path under the site's docs folder.
		urls := make([]string, 0, len(pages))
		for u := range pages {
			urls = append(urls, u)
		}
		sort.Strings(urls)
		for _, u := range urls {
			if !underAny(u, s.Skip) {
				out[s.SkillRef+"/docs/"+pages[u].file] = pages[u].body
			}
		}
		for ref, src := range s.Extra {
			raw, err := os.ReadFile(filepath.Join(root, src))
			if err != nil {
				return err
			}
			out[s.SkillRef+"/"+ref] = generatedNote(src) + absLinks(string(raw), s.URL)
		}

		// llms.txt: nav order first, then any page the nav doesn't list.
		var idx, full strings.Builder
		fmt.Fprintf(&idx, "# %s\n\n> %s\n\n%s\n\nEvery page below in one file: %s/llms-full.txt\n", s.Name, s.Summary, s.Intro, s.URL)
		fmt.Fprintf(&full, "# %s\n\n> %s\n\n%s\n", s.Name, s.Summary, s.Intro)
		seen := map[string]bool{}
		writePage := func(u string) {
			p := pages[u]
			fmt.Fprintf(&full, "\n\n---\n\nSource: %s%s\n\n%s", s.URL, u, strings.TrimSpace(stripGenerated(p.body))+"\n")
		}
		for _, sec := range sections {
			var lines []string
			for _, l := range sec.links {
				switch {
				case strings.HasPrefix(l.url, "/docs"):
					p, ok := pages[l.url]
					if !ok || seen[l.url] {
						continue
					}
					seen[l.url] = true
					lines = append(lines, entry(l.title, s.URL+l.url, p.desc))
					writePage(l.url)
				case strings.Contains(l.url, "://"):
					lines = append(lines, entry(l.title, l.url, ""))
				}
			}
			if len(lines) > 0 {
				fmt.Fprintf(&idx, "\n## %s\n\n%s\n", sec.title, strings.Join(lines, "\n"))
			}
		}
		var rest []string
		for _, u := range urls {
			if !seen[u] {
				rest = append(rest, entry(pages[u].title, s.URL+u, pages[u].desc))
				writePage(u)
			}
		}
		if len(rest) > 0 {
			fmt.Fprintf(&idx, "\n## Optional\n\n%s\n", strings.Join(rest, "\n"))
		}
		out[s.Pages+"/llms.txt"] = idx.String()
		out[s.Pages+"/llms-full.txt"] = full.String()
	}
	return nil
}

func entry(title, url, desc string) string {
	if desc != "" {
		return fmt.Sprintf("- [%s](%s): %s", title, url, desc)
	}
	return fmt.Sprintf("- [%s](%s)", title, url)
}

type docsPage struct{ title, desc, body, file string } // file: path under the docs folder, .md

// docsPages reads every page under <Pages>/(main)/docs (not _private, not +files), keyed by its URL path. Each body is
// plain Markdown: the site's front matter and EJS escapes removed, site links made absolute, the title as an H1.
func docsPages(root string, s docsSite, generated map[string]string) (map[string]docsPage, error) {
	docs := s.Pages + "/(main)/docs"
	raw := map[string]string{} // repo path -> content
	err := filepath.WalkDir(filepath.Join(root, docs), func(full string, d fs.DirEntry, err error) error {
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
		b, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		content := string(b)
		if strings.HasSuffix(n, ".ejs") {
			content = includeIn(root, s.Pages, path.Dir(rel), content)
			if strings.Contains(content, "<%") { // a real template, not a Markdown wrapper: nothing to copy
				return nil
			}
		} else {
			// A Markdown page that runs code (live data, as /docs/compatibility does): the copy keeps the prose only.
			content = liveEJSRe.ReplaceAllString(content, "")
		}
		raw[rel] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	for p, content := range generated {
		if strings.HasPrefix(p, docs+"/") && strings.HasSuffix(p, ".md") {
			raw[p] = content
		}
	}
	out := map[string]docsPage{}
	for p, content := range raw {
		u := "/docs" + strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(p, docs), ".md"), ".ejs")
		u = strings.TrimSuffix(u, "/index")
		fm, body := splitFront(content)
		body = unescapeEJS(body)
		body = generatedComment.ReplaceAllString(body, "")
		body = absLinks(strings.TrimLeft(body, "\n"), s.URL)
		title := fm["title"]
		if m := h1Re.FindStringSubmatch(firstLine(body)); m != nil {
			if title == "" {
				title = stripMarkdown(m[1])
			}
		} else if title != "" {
			body = "# " + title + "\n\n" + body
		}
		src := p
		if v := fm["source"]; v != "" {
			src = v
		}
		file := strings.TrimPrefix(p, docs+"/")
		file = strings.TrimSuffix(file, ".ejs")
		if !strings.HasSuffix(file, ".md") {
			file += ".md"
		}
		out[u] = docsPage{title: title, desc: fm["description"], body: generatedNote(src) + body, file: file}
	}
	return out, nil
}

// includeIn inlines `<%- include('name') %>` from the nearest _private folder, as the site does.
func includeIn(root, pages, dir, content string) string {
	return includeRe.ReplaceAllStringFunc(content, func(s string) string {
		name := includeRe.FindStringSubmatch(s)[1]
		if !strings.HasSuffix(name, ".md") {
			name += ".md"
		}
		for d := dir; strings.HasPrefix(d, pages); d = path.Dir(d) {
			if raw, err := os.ReadFile(filepath.Join(root, d, "_private", name)); err == nil {
				return string(raw)
			}
		}
		return s
	})
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

const generatedPrefix = "<!-- Generated by site/sync from "

func generatedNote(src string) string {
	return generatedPrefix + src + ". Edit that file, then run site/sync. -->\n"
}

func stripGenerated(s string) string {
	if strings.HasPrefix(s, generatedPrefix) {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			return s[i+1:]
		}
	}
	return s
}

var (
	generatedComment = regexp.MustCompile(`(?m)^<!-- Generated by site/sync[^\n]*-->\n`)
	frontLineRe      = regexp.MustCompile(`^([A-Za-z_]+):\s*(.*)$`)
	siteLinkRe       = regexp.MustCompile(`\]\((/(?:docs|blog|llms)[^)\s]*)`)
	siteRefRe        = regexp.MustCompile(`(?m)^(\s*\[[^\]]+\]:\s*)(/(?:docs|blog|llms)\S*)`)
	scriptServerEsc  = "<%- '<' %>"
)

// splitFront returns a page's simple key: value front matter (quoted values unquoted) and the rest.
func splitFront(s string) (map[string]string, string) {
	fm := map[string]string{}
	m := frontStripRe.FindString(s)
	if m == "" {
		return fm, s
	}
	for _, line := range strings.Split(m, "\n") {
		if p := frontLineRe.FindStringSubmatch(line); p != nil {
			v := strings.TrimSpace(p[2])
			if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
				v = strings.ReplaceAll(v[1:len(v)-1], `\"`, `"`)
			}
			fm[p[1]] = v
		}
	}
	return fm, s[len(m):]
}

// liveEJSRe matches a real EJS tag in a Markdown page, never an escaped "<%%" (a literal "<%").
var liveEJSRe = regexp.MustCompile(`(?s)<%[^%].*?%>`)

// unescapeEJS undoes escapeEJS: the copies are read as Markdown, never run as templates.
func unescapeEJS(s string) string {
	s = strings.ReplaceAll(s, scriptServerEsc, "<")
	return strings.ReplaceAll(s, "<%%", "<%")
}

// absLinks points site-relative links (/docs/..., /blog/...) at the live site, so they work outside it.
func absLinks(s, base string) string {
	s = siteLinkRe.ReplaceAllString(s, "]("+base+"$1")
	return siteRefRe.ReplaceAllString(s, "${1}"+base+"$2")
}

type tocSection struct {
	title string
	links []tocLink
}

type tocLink struct{ title, url string }

var tocLinkRe = regexp.MustCompile(`^\s*-\s*\[([^\]]+)\]\(([^)\s]+)\)`)

func parseToc(s string) []tocSection {
	var out []tocSection
	for _, line := range strings.Split(s, "\n") {
		if m := h1Re.FindStringSubmatch(line); m != nil {
			out = append(out, tocSection{title: m[1]})
			continue
		}
		if m := tocLinkRe.FindStringSubmatch(line); m != nil {
			if len(out) == 0 {
				out = append(out, tocSection{title: "Docs"})
			}
			out[len(out)-1].links = append(out[len(out)-1].links, tocLink{title: m[1], url: m[2]})
		}
	}
	return out
}
