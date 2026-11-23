package llmsearch

import (
	"bytes"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Document kinds: they pick the splitter.
const (
	KindMarkdown = "markdown"
	KindCode     = "code"
	KindText     = "text"
)

// Chunk sizes, in bytes (about four per token): pieces are packed up to ChunkTarget and never pass ChunkMax.
const (
	ChunkTarget = 2000
	ChunkMax    = 4000
)

var codeExts = map[string]bool{
	".go": true, ".js": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true, ".jsx": true, ".py": true,
	".rb": true, ".rs": true, ".java": true, ".kt": true, ".swift": true, ".c": true, ".h": true, ".cc": true,
	".cpp": true, ".hpp": true, ".cs": true, ".php": true, ".sh": true, ".bash": true, ".zsh": true, ".sql": true,
	".lua": true, ".dart": true, ".scala": true, ".svelte": true, ".vue": true, ".css": true, ".scss": true,
	".html": true, ".ejs": true, ".ets": true, ".yaml": true, ".yml": true, ".toml": true, ".json": true,
}

// kindOf is the splitter for a document name.
func kindOf(name string) string {
	ext := strings.ToLower(path.Ext(name))
	switch {
	case ext == ".md" || ext == ".mdx" || ext == ".markdown":
		return KindMarkdown
	case codeExts[ext]:
		return KindCode
	}
	return KindText
}

// isText reports whether b is UTF-8 text without NUL bytes.
func isText(b []byte) bool {
	return utf8.Valid(b) && bytes.IndexByte(b, 0) < 0
}

// tokens estimates a text's size in tokens.
func tokens(s string) int { return (len(s) + 3) / 4 }

// piece is one chunk before it's stored.
type piece struct {
	Heading    string
	Anchor     string // the id of the section's heading (H2-H6), as a Markdown renderer gives it; "" for the top
	Start, End int    // 1-based lines, inclusive
	Text       string
}

// para is a block of lines.
type para struct {
	start, end int
	text       string
}

// split cuts a document into chunks.
func split(kind, text string) []piece {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if kind == KindMarkdown {
		return splitMarkdown(lines)
	}
	return pack("", paragraphs(lines, 0, len(lines)))
}

var headingRe = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)

// splitMarkdown cuts at headings; each chunk carries its heading path ("Title › Section").
func splitMarkdown(lines []string) []piece {
	var out []piece
	var stack []string // the heading at each level, "" where skipped
	title := ""        // the front matter's title, leading every path
	from := 0
	// front matter
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if t := strings.TrimSpace(lines[i]); t == "---" || t == "..." {
				for _, l := range lines[1:i] {
					if v, ok := strings.CutPrefix(l, "title:"); ok {
						title = strings.Trim(strings.TrimSpace(v), `"'`)
					}
				}
				from = i + 1
				break
			}
		}
	}
	heading := func() string {
		var parts []string
		if title != "" && (len(stack) == 0 || stack[0] != title) {
			parts = append(parts, title)
		}
		for _, h := range stack {
			if h != "" {
				parts = append(parts, h)
			}
		}
		return strings.Join(parts, " › ")
	}
	start, fence := from, ""
	anchor, seen := "", map[string]bool{}
	flush := func(end int) {
		for _, p := range pack(heading(), paragraphs(lines, start, end)) {
			p.Anchor = anchor
			out = append(out, p)
		}
	}
	for i := from; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if fence != "" {
			if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = t[:3]
			continue
		}
		m := headingRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		flush(i)
		level := len(m[1])
		if id := headingID(headingText(m[2]), seen); level > 1 {
			anchor = id
		} else {
			anchor = ""
		}
		if len(stack) >= level {
			stack = stack[:level-1]
		}
		for len(stack) < level-1 {
			stack = append(stack, "")
		}
		stack = append(stack, m[2])
		start = i + 1
	}
	flush(len(lines))
	return out
}

// paragraphs are lines[from:to] split at blank lines (outside fenced code), with their line numbers.
func paragraphs(lines []string, from, to int) []para {
	var out []para
	var cur []string
	curStart, fence := 0, ""
	end := func(i int) {
		if len(cur) > 0 {
			out = append(out, para{start: curStart + 1, end: i, text: strings.Join(cur, "\n")})
			cur = nil
		}
	}
	for i := from; i < to; i++ {
		l := lines[i]
		t := strings.TrimSpace(l)
		if fence == "" && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")) {
			fence = t[:3]
		} else if fence != "" && strings.HasPrefix(t, fence) {
			fence = ""
		} else if fence == "" && t == "" {
			end(i)
			continue
		}
		if len(cur) == 0 {
			curStart = i
		}
		cur = append(cur, strings.TrimRight(l, " \t"))
	}
	end(to)
	return out
}

// pack joins paragraphs into chunks of about ChunkTarget bytes, cutting any longer than ChunkMax by lines (and a
// line longer than that by bytes).
func pack(heading string, paras []para) []piece {
	var out []piece
	var cur *piece
	emit := func() {
		if cur != nil && strings.TrimSpace(cur.Text) != "" {
			out = append(out, *cur)
		}
		cur = nil
	}
	for _, p := range cutLong(paras) {
		if cur != nil && len(cur.Text)+2+len(p.text) > ChunkTarget {
			emit()
		}
		if cur == nil {
			cur = &piece{Heading: heading, Start: p.start, End: p.end, Text: p.text}
			continue
		}
		cur.Text += "\n\n" + p.text
		cur.End = p.end
	}
	emit()
	return out
}

// cutLong splits paragraphs over ChunkMax.
func cutLong(paras []para) []para {
	var out []para
	for _, p := range paras {
		if len(p.text) <= ChunkMax {
			out = append(out, p)
			continue
		}
		var cur []string
		size, start := 0, p.start
		for i, l := range strings.Split(p.text, "\n") {
			line := p.start + i
			for len(l) > ChunkMax { // one very long line
				if len(cur) > 0 {
					out = append(out, para{start: start, end: line - 1, text: strings.Join(cur, "\n")})
					cur, size = nil, 0
				}
				cut := ChunkMax
				for cut > 0 && !utf8.RuneStart(l[cut]) {
					cut--
				}
				out = append(out, para{start: line, end: line, text: l[:cut]})
				l = l[cut:]
				start = line
			}
			if size+len(l)+1 > ChunkMax && len(cur) > 0 {
				out = append(out, para{start: start, end: line - 1, text: strings.Join(cur, "\n")})
				cur, size = nil, 0
			}
			if len(cur) == 0 {
				start = line
			}
			cur = append(cur, l)
			size += len(l) + 1
		}
		if len(cur) > 0 {
			out = append(out, para{start: start, end: p.end, text: strings.Join(cur, "\n")})
		}
	}
	return out
}

// embedText is what's embedded for a chunk: its section's heading path, then its text, so a chunk that says "Run it
// with --dir" still matches a question about the section it's in. The document's name is left out, so the same content
// embeds the same wherever it lives (a loose file, the same file in a zip, another bucket) and its vector is reused.
func embedText(p piece) string {
	if p.Heading == "" {
		return p.Text
	}
	return p.Heading + "\n\n" + p.Text
}

var (
	mdImageRe = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	mdLinkRe  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdTagRe   = regexp.MustCompile(`(?s)<[^>]+>`)
)

// headingText is a heading's plain text, as a Markdown renderer sees it for the heading's id: links and images as
// their text, no tags, code marks or bold.
func headingText(s string) string {
	s = mdImageRe.ReplaceAllString(s, "$1")
	s = mdLinkRe.ReplaceAllString(s, "$1")
	s = mdTagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(strings.NewReplacer("`", "", "**", "", "__", "").Replace(s))
}

// headingID is goldmark's automatic heading id (what PocketOne's Pages and most Markdown sites render): ASCII letters,
// digits and _ lowercased; a run of spaces and - becomes one -; the rest (non-ASCII too) drops; a repeat on the page
// gets -1, -2, ...
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
			if !strings.HasSuffix(b.String(), "-") {
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
		n := id + "-" + strconv.Itoa(i)
		if !seen[n] {
			seen[n] = true
			return n
		}
	}
}
