package main

// Heading ids: the id the site (goldmark) gives a heading, so generated pages can link to generated headings.

import (
	"fmt"
	"strings"
)

// headingID is goldmark's id for a heading's text, made unique within a page by seen.
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

// gid is the id the site gives a heading with this text (anchors to generated headings use it).
func gid(text string) string { return headingID(text, map[string]bool{}) }
