package search

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"golang.org/x/net/html"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	searchv1 "pocketone.dev/ext/contracts/search/v1"
)

// Fetch limits.
var (
	FetchTimeout      = 30 * time.Second
	MaxMultiFileBytes = int64(1 << 20) // a file in a folder, zip or repository: larger ones are skipped (generated, data)
)

// httpClient fetches pages on Go's default transport, so a multitenant host's egress guard applies to it.
var httpClient = &http.Client{Timeout: FetchTimeout}

// skipDirs and skipFiles are left out of every folder, zip and repository.
var (
	skipDirs  = map[string]bool{"node_modules": true, "vendor": true, "__pycache__": true}
	skipFiles = map[string]bool{"package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true, "bun.lockb": true,
		"go.sum": true, "Cargo.lock": true, "composer.lock": true, "Gemfile.lock": true, "poetry.lock": true, "uv.lock": true}
)

// skipPath reports whether a file of a multi-file source is left out: in a dot folder (.git, .github) or one of
// skipDirs, a dotfile or lockfile, or matching one of exclude (against the whole path, the file name or any folder).
func skipPath(p string, exclude []string) bool {
	p = strings.TrimPrefix(strings.ReplaceAll(p, `\`, "/"), "/")
	if p == "" {
		return true
	}
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ".") || (i < len(parts)-1 && skipDirs[part]) {
			return true
		}
	}
	if skipFiles[parts[len(parts)-1]] {
		return true
	}
	for _, pat := range exclude {
		pat = strings.Trim(pat, "/")
		if pat == "" {
			continue
		}
		if ok, _ := path.Match(pat, p); ok || strings.HasPrefix(p, pat+"/") {
			return true
		}
		for _, part := range parts {
			if ok, _ := path.Match(pat, part); ok {
				return true
			}
		}
	}
	return false
}

// readPage fetches a page source's URL and keeps it as one document. An unchanged page (its ETag, or its text) costs
// nothing.
func (s *service) readPage(b, src *core.Record, force bool) (string, error) {
	sp := specOf(src)
	ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sp.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "PocketOne-search/1")
	req.Header.Set("Accept", "text/markdown, text/plain;q=0.9, text/html;q=0.8, */*;q=0.1")
	etag := src.GetString("state")
	if etag != "" && !force {
		req.Header.Set("If-None-Match", etag)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", sp.URL, err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotModified {
		return etag, nil
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching %s: %s", sp.URL, res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, MaxFileBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > MaxFileBytes {
		return "", tooLarge(fmt.Sprintf("%s is over %d MB", sp.URL, MaxFileBytes>>20))
	}
	ctype, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
	text, kind := "", ""
	switch {
	case ctype == "text/html" || ctype == "application/xhtml+xml":
		text, kind = htmlToMarkdown(string(body)), KindMarkdown
	case ctype == "text/markdown" || strings.HasSuffix(strings.ToLower(req.URL.Path), ".md") || strings.HasSuffix(req.URL.Path, "/llms.txt") || strings.HasSuffix(req.URL.Path, "/llms-full.txt"):
		text, kind = string(body), KindMarkdown
	case isText(body):
		text = string(body)
		kind = kindOf(req.URL.Path)
	default:
		return "", fmt.Errorf("%s isn't text (%s)", sp.URL, ctype)
	}
	if err := s.syncDocs(b, src, []docIn{{name: sp.URL, text: text, kind: kind}}, force); err != nil {
		return "", err
	}
	return res.Header.Get("ETag"), nil
}

// skipElements are left out of a page's text.
var skipElements = map[string]bool{"script": true, "style": true, "noscript": true, "svg": true, "nav": true,
	"footer": true, "template": true, "iframe": true, "form": true, "button": true, "head": false}

// htmlToMarkdown is a page's readable text as Markdown: headings as #, list items as -, code blocks fenced, the
// title as front matter; scripts, styles, navigation and footers left out.
func htmlToMarkdown(src string) string {
	z := html.NewTokenizer(strings.NewReader(src))
	var b strings.Builder
	var title string
	skip, inTitle, inPre := 0, false, 0
	block := func(prefix string) {
		s := b.String()
		if s != "" && !strings.HasSuffix(s, "\n\n") {
			if strings.HasSuffix(s, "\n") {
				b.WriteString("\n")
			} else {
				b.WriteString("\n\n")
			}
		}
		b.WriteString(prefix)
	}
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			out := strings.TrimSpace(collapseBlankLines(b.String()))
			if title != "" {
				out = "---\ntitle: " + strings.ReplaceAll(strings.TrimSpace(title), "\n", " ") + "\n---\n\n" + out
			}
			return out + "\n"
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)
			start := tt != html.EndTagToken
			if skipElements[tag] {
				if tt == html.StartTagToken {
					skip++
				} else if tt == html.EndTagToken && skip > 0 {
					skip--
				}
				continue
			}
			if skip > 0 {
				continue
			}
			switch tag {
			case "title":
				inTitle = start
			case "h1", "h2", "h3", "h4", "h5", "h6":
				if start {
					block(strings.Repeat("#", int(tag[1]-'0')) + " ")
				} else {
					b.WriteString("\n\n")
				}
			case "p", "div", "section", "article", "main", "table", "tr", "blockquote", "ul", "ol", "dl", "dt", "dd":
				block("")
			case "li":
				if start {
					block("- ")
				}
			case "br":
				b.WriteString("\n")
			case "pre":
				if start {
					inPre++
					block("```\n")
				} else if inPre > 0 {
					inPre--
					b.WriteString("\n```\n\n")
				}
			case "td", "th":
				if start {
					b.WriteString(" | ")
				}
			}
		case html.TextToken:
			if skip > 0 {
				continue
			}
			t := string(z.Text())
			if inTitle {
				title += t
				continue
			}
			if inPre > 0 {
				b.WriteString(t)
				continue
			}
			t = strings.Join(strings.Fields(t), " ")
			if t == "" {
				continue
			}
			s := b.String()
			if s != "" && !strings.HasSuffix(s, " ") && !strings.HasSuffix(s, "\n") && !strings.HasSuffix(s, "# ") && !strings.HasSuffix(s, "- ") {
				b.WriteString(" ")
			}
			b.WriteString(t)
		}
	}
}

func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, l := range lines {
		l = strings.TrimRight(l, " ")
		if strings.TrimSpace(l) == "" {
			blank++
			if blank > 1 {
				continue
			}
			l = ""
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

var _ = searchv1.KindPage
