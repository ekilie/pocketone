package main

import (
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

// chunk is one passage of the knowledge: a section of a page, small enough to hand the model a few of them.
type chunk struct {
	Title  string // "Page title › Section"
	Source string // the page's URL, when the file names one
	Text   string
	terms  map[string]int
	length int
}

// knowledge is an llms-full.txt split into passages, searchable with BM25.
type knowledge struct {
	chunks []chunk
	df     map[string]int
	avgLen float64
}

const maxChunk = 1400 // characters per passage

// loadKnowledge reads llms-full.txt files or URLs (llms.txt format: pages separated by "---", each with an optional
// "Source: <url>" line and a "# Title").
func loadKnowledge(sources []string) (*knowledge, error) {
	k := &knowledge{df: map[string]int{}}
	for _, src := range sources {
		text, err := readSource(src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", src, err)
		}
		k.add(text)
	}
	if len(k.chunks) == 0 {
		return nil, fmt.Errorf("no passages in %s", strings.Join(sources, ", "))
	}
	total := 0
	for i := range k.chunks {
		c := &k.chunks[i]
		c.terms = map[string]int{}
		for _, w := range words(c.Title + " " + c.Text) {
			c.terms[w]++
			c.length++
		}
		for w := range c.terms {
			k.df[w]++
		}
		total += c.length
	}
	k.avgLen = float64(total) / float64(len(k.chunks))
	return k, nil
}

func readSource(src string) (string, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		c := &http.Client{Timeout: 30 * time.Second}
		res, err := c.Get(src)
		if err != nil {
			return "", err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("%s", res.Status)
		}
		b, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
		return string(b), err
	}
	if st, err := os.Stat(src); err == nil && st.IsDir() {
		return readDir(src)
	}
	b, err := os.ReadFile(src)
	return string(b), err
}

// readDir turns a folder of Markdown pages into llms-full.txt form: each .md file one page, titled by its front
// matter's title (else its first heading, else its path).
func readDir(dir string) (string, error) {
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text, title := string(raw), ""
		if rest, ok := strings.CutPrefix(text, "---\n"); ok {
			if fm, body, ok := strings.Cut(rest, "\n---\n"); ok {
				text = body
				for _, line := range strings.Split(fm, "\n") {
					if v, ok := strings.CutPrefix(line, "title:"); ok {
						title = strings.Trim(strings.TrimSpace(v), `"'`)
					}
				}
			}
		}
		rel, _ := filepath.Rel(dir, path)
		if title == "" {
			title = rel
		}
		fmt.Fprintf(&b, "\n---\n\n# %s\n\n%s\n", title, text)
		return nil
	})
	return b.String(), err
}

// add splits one file into pages, pages into sections, and long sections at paragraph breaks.
func (k *knowledge) add(text string) {
	for _, page := range strings.Split(text, "\n---\n") {
		var source, title string
		var section string
		var buf strings.Builder
		flush := func() {
			body := strings.TrimSpace(buf.String())
			buf.Reset()
			if body == "" {
				return
			}
			name := title
			if section != "" && section != title {
				name = strings.TrimSpace(title + " › " + section)
			}
			for _, part := range splitLong(body) {
				k.chunks = append(k.chunks, chunk{Title: name, Source: source, Text: part})
			}
		}
		fenced := false // inside a ``` block, "# ..." is code (a shell or YAML comment), not a heading
		for _, line := range strings.Split(page, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				fenced = !fenced
			}
			switch {
			case fenced:
				buf.WriteString(line)
				buf.WriteByte('\n')
			case strings.HasPrefix(line, "Source: ") && source == "":
				source = strings.TrimSpace(strings.TrimPrefix(line, "Source: "))
			case strings.HasPrefix(line, "# "):
				flush()
				title, section = strings.TrimSpace(line[2:]), ""
			case strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### "):
				flush()
				section = strings.TrimSpace(strings.TrimLeft(line, "# "))
			default:
				buf.WriteString(line)
				buf.WriteByte('\n')
			}
		}
		flush()
	}
}

func splitLong(body string) []string {
	if len(body) <= maxChunk {
		return []string{body}
	}
	var out []string
	var cur strings.Builder
	for _, para := range strings.Split(body, "\n\n") {
		if cur.Len() > 0 && cur.Len()+len(para) > maxChunk {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
		if len(para) > maxChunk {
			para = para[:maxChunk]
		}
		cur.WriteString(para)
		cur.WriteString("\n\n")
	}
	if strings.TrimSpace(cur.String()) != "" {
		out = append(out, strings.TrimSpace(cur.String()))
	}
	return out
}

// search returns the n passages that best match query (BM25).
func (k *knowledge) search(query string, n int) []chunk {
	const k1, b = 1.2, 0.75
	q := words(query)
	if len(q) == 0 {
		return nil
	}
	type scored struct {
		i int
		s float64
	}
	var hits []scored
	N := float64(len(k.chunks))
	for i, c := range k.chunks {
		var s float64
		for _, w := range q {
			tf := float64(c.terms[w])
			if tf == 0 {
				continue
			}
			df := float64(k.df[w])
			idf := math.Log(1 + (N-df+0.5)/(df+0.5))
			s += idf * tf * (k1 + 1) / (tf + k1*(1-b+b*float64(c.length)/k.avgLen))
		}
		if s > 0 {
			hits = append(hits, scored{i, s})
		}
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].s > hits[b].s })
	var out []chunk
	for _, h := range hits {
		if len(out) == n {
			break
		}
		out = append(out, k.chunks[h.i])
	}
	return out
}

var stop = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a an and are as at be but by can do does for from has have how i if in into is it its
		me my of on or so that the their them then there these this to was we what when where which who why will with you your
		about any all also not no yes up out use using get set one our us should would could just like more than very`) {
		stop[w] = true
	}
}

// words lowercases text into search terms, dropping stop words and single letters.
func words(s string) []string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	out := f[:0]
	for _, w := range f {
		if len(w) > 1 && !stop[w] {
			out = append(out, w)
		}
	}
	return out
}
