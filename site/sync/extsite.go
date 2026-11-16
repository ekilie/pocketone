package main

// Extension sites. An extension may carry end-user docs in PocketPages format beside its README:
//
//	ext/<name>/site/            mounted at /docs/extensions/<name>/ (the README stays the landing page)
//	ext/<name>/site/_private/toc.md   its nav, spliced into the docs nav after the extension's group
//	ext/<name>/blog/*.md        posts merged into the pocketone.dev blog (/blog/<file name>)
//
// Unlike a README, these files are pages-native: EJS runs, so a code sample writes "<%%" for a literal "<%". Sync
// copies them, adding `source:` to each Markdown page's front matter (the "Edit this page" link) and pointing its
// relative links at site URLs (a link is relative to the file, as on GitHub: `ctx/req.md`, `../routing.md`, an image
// next to the page). A relative link to a file that doesn't exist fails the sync. The subtree is content only: no
// `+` files (layouts, config, middleware, API routes) and no index page at its root, where the README is.

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

const blogOut = "site/pb_pages/(main)/blog"

// siteToc is the extension site's nav file, relative to ext/<name>/site; sync reads it and doesn't copy it.
const siteToc = "_private/toc.md"

// loadSite lists ext/<name>/site (paths relative to it) and ext/<name>/blog/*.md (file names), with what's wrong.
func loadSite(root string, p *extension) []string {
	var problems []string
	base := filepath.Join(root, "ext", p.Name, "site")
	if st, err := os.Stat(base); err == nil && st.IsDir() {
		_ = filepath.WalkDir(base, func(full string, d fs.DirEntry, err error) error {
			if err != nil {
				problems = append(problems, err.Error())
				return nil
			}
			rel, _ := filepath.Rel(base, full)
			rel = filepath.ToSlash(rel)
			n := d.Name()
			if d.IsDir() || n == ".DS_Store" {
				return nil
			}
			where := "ext/" + p.Name + "/site/" + rel
			switch {
			case strings.HasPrefix(n, "+"):
				problems = append(problems, where+": an extension site is content only (no + files); the pocketone.dev site owns layouts, config, middleware and API routes")
			case !strings.Contains(rel, "/") && strings.HasPrefix(n, "index."):
				problems = append(problems, where+": the extension's README is the page at /docs/extensions/"+p.Name+"; no index page at the root of site/")
			}
			p.site = append(p.site, rel)
			return nil
		})
		sort.Strings(p.site)
		if !contains(p.site, siteToc) {
			problems = append(problems, "ext/"+p.Name+"/site: needs "+siteToc+" (its nav)")
		}
		for _, g := range p.guides {
			if contains(p.site, g.slug+".md") || contains(p.site, g.slug+"/index.md") {
				problems = append(problems, fmt.Sprintf("ext/%s: %s and site/%s.md are both /docs/extensions/%s/%s", p.Name, path.Base(g.src), g.slug, p.Name, g.slug))
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "ext", p.Name, "blog"))
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				p.blog = append(p.blog, e.Name())
			}
		}
	}
	return problems
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// sitePageURL maps a page in an extension site (relative to site/) to its URL, or "" for a file that isn't a page.
func sitePageURL(name, rel string) string {
	if !strings.HasSuffix(rel, ".md") && !strings.HasSuffix(rel, ".ejs") || strings.HasPrefix(rel, "_private/") || strings.Contains(rel, "/_private/") {
		return ""
	}
	u := strings.TrimSuffix(strings.TrimSuffix(rel, ".md"), ".ejs")
	u = strings.TrimSuffix(strings.TrimSuffix(u, "index"), "/")
	if u == "" {
		return "/docs/extensions/" + name
	}
	return "/docs/extensions/" + name + "/" + u
}

// addSitePages adds every extension site's pages, files and blog posts to pages (repo path -> site URL), so links
// from anywhere (READMEs included) reach them.
func addSitePages(m *manifest, pages map[string]string) {
	for _, p := range m.Extensions {
		for _, rel := range p.site {
			src := "ext/" + p.Name + "/site/" + rel
			if u := sitePageURL(p.Name, rel); u != "" {
				pages[src] = u
				if d, f := path.Split(src); strings.HasPrefix(f, "index.") {
					pages[strings.TrimSuffix(d, "/")] = u
				}
			} else if !strings.Contains(rel, "_private/") {
				pages[src] = "/docs/extensions/" + p.Name + "/" + rel // a file served next to the pages (images)
			}
		}
		for _, f := range p.blog {
			pages["ext/"+p.Name+"/blog/"+f] = "/blog/" + strings.TrimSuffix(f, ".md")
		}
	}
}

// siteFiles returns the generated files for every extension site and blog: repo path -> content.
func siteFiles(root string, m *manifest, rw *rewriter) (map[string]string, error) {
	out := map[string]string{}
	var broken []string
	for _, p := range m.Extensions {
		for _, rel := range p.site {
			if rel == siteToc {
				continue
			}
			src := "ext/" + p.Name + "/site/" + rel
			raw, err := os.ReadFile(filepath.Join(root, src))
			if err != nil {
				return nil, err
			}
			dst := extensionsOut + "/" + p.Name + "/" + rel
			if strings.HasSuffix(rel, ".md") && !strings.Contains(rel, "_private/") {
				body, bad := rw.sitePage(src, string(raw))
				broken = append(broken, bad...)
				out[dst] = withSource(body, src)
				continue
			}
			out[dst] = string(raw) // templates, partials and files (images) as they are
		}
		for _, f := range p.blog {
			src := "ext/" + p.Name + "/blog/" + f
			raw, err := os.ReadFile(filepath.Join(root, src))
			if err != nil {
				return nil, err
			}
			dst := blogOut + "/" + f
			if have, err := os.ReadFile(filepath.Join(root, dst)); err == nil && !strings.Contains(string(have), generatedPrefix) {
				return nil, fmt.Errorf("%s: %s is a hand-written post with the same name", src, dst)
			}
			body, bad := rw.sitePage(src, string(raw))
			broken = append(broken, bad...)
			out[dst] = withSource(body, src)
		}
	}
	if len(broken) > 0 {
		sort.Strings(broken)
		return nil, fmt.Errorf("broken links in extension sites:\n  %s", strings.Join(broken, "\n  "))
	}
	return out, nil
}

// sitePage points a pages-native Markdown file's relative links at site URLs (no EJS escaping: it's a template), and
// lists the relative links that reach no file.
func (r *rewriter) sitePage(src, content string) (string, []string) {
	var broken []string
	target := func(t string, image bool) string {
		if t == "" || strings.HasPrefix(t, "#") || strings.Contains(t, "://") || strings.HasPrefix(t, "mailto:") || strings.HasPrefix(t, "/") || strings.Contains(t, "<%") {
			return t
		}
		clean := t
		if i := strings.Index(clean, "#"); i >= 0 {
			clean = clean[:i]
		}
		p := path.Clean(path.Join(path.Dir(src), clean))
		if _, ok := r.pages[p]; !ok {
			if _, err := os.Stat(filepath.Join(r.root, p)); err != nil {
				broken = append(broken, fmt.Sprintf("%s: %s", src, t))
				return t
			}
		}
		return r.target(src, t, image)
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	fence := false
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		if m := refRe.FindStringSubmatch(l); m != nil {
			lines[i] = m[1] + target(m[2], false) + m[3]
			continue
		}
		lines[i] = r.outsideCode(l, func(s string) string {
			return inlineRe.ReplaceAllStringFunc(s, func(match string) string {
				p := inlineRe.FindStringSubmatch(match)
				return p[1] + "[" + p[2] + "](" + target(p[3], p[1] == "!") + p[4] + ")"
			})
		})
	}
	return strings.Join(lines, "\n"), broken
}

var frontEndRe = regexp.MustCompile(`(?s)\A---\n.*?\n---\n`)

// withSource adds `source: <src>` to a page's front matter (the "Edit this page" link) and the generated-file note.
func withSource(body, src string) string {
	q := `"` + strings.ReplaceAll(src, `"`, `\"`) + `"`
	if fm := frontEndRe.FindString(body); fm != "" {
		return fm[:len(fm)-4] + "source: " + q + "\n---\n" + generatedNote(src) + body[len(fm):]
	}
	return "---\nsource: " + q + "\n---\n" + generatedNote(src) + body
}

// siteNav is an extension site's nav (its _private/toc.md) with its links pointed at site URLs and each section titled
// "<extension>: <section>".
func siteNav(root string, p *extension, rw *rewriter) (string, error) {
	src := "ext/" + p.Name + "/site/" + siteToc
	raw, err := os.ReadFile(filepath.Join(root, src))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	var broken []string
	for _, l := range strings.Split(string(raw), "\n") {
		if m := h1Re.FindStringSubmatch(l); m != nil {
			fmt.Fprintf(&b, "\n# %s: %s\n\n", p.Title, m[1])
			continue
		}
		if m := tocLinkRe.FindStringSubmatch(l); m != nil {
			u := m[2]
			if !strings.Contains(u, "://") && !strings.HasPrefix(u, "/") {
				frag := ""
				if i := strings.Index(u, "#"); i >= 0 {
					u, frag = u[:i], u[i:]
				}
				site, ok := rw.pages[path.Clean(path.Join(path.Dir(src), u))]
				if !ok {
					broken = append(broken, src+": "+m[2])
					continue
				}
				u = site + frag
			}
			fmt.Fprintf(&b, "- [%s](%s)\n", m[1], u)
		}
	}
	if len(broken) > 0 {
		return "", fmt.Errorf("broken links in extension site navs:\n  %s", strings.Join(broken, "\n  "))
	}
	return b.String(), nil
}
