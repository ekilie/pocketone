package search

import (
	"strings"
	"testing"
)

func TestKindOf(t *testing.T) {
	for name, want := range map[string]string{"a/b.md": KindMarkdown, "x.MDX": KindMarkdown, "main.go": KindCode,
		"notes.txt": KindText, "README": KindText, "app.ets": KindCode} {
		if got := kindOf(name); got != want {
			t.Errorf("%s: %s, want %s", name, got, want)
		}
	}
	if isText([]byte("a\x00b")) || isText([]byte{0xff, 0xfe}) || !isText([]byte("héllo")) {
		t.Fatal("isText")
	}
}

// Markdown splits at headings (not inside fenced code), each chunk with its heading path and lines.
func TestSplitMarkdown(t *testing.T) {
	doc := `---
title: "Guide"
---
Intro text.

# Install

Get it.

## macOS

` + "```sh\n# not a heading\nbrew install x\n```" + `

### Notes
Some notes.

## Linux

Use the script.
`
	got := split(KindMarkdown, doc)
	var heads []string
	for _, p := range got {
		heads = append(heads, p.Heading)
	}
	want := "Guide|Guide › Install|Guide › Install › macOS|Guide › Install › macOS › Notes|Guide › Install › Linux"
	if strings.Join(heads, "|") != want {
		t.Fatalf("headings %q", heads)
	}
	if !strings.Contains(got[2].Text, "# not a heading") || got[2].Start != 12 || got[2].End != 15 {
		t.Fatalf("macOS chunk: %+v", got[2])
	}
	if got[0].Text != "Intro text." || got[0].Start != 4 {
		t.Fatalf("intro: %+v", got[0])
	}
}

// Paragraphs pack up to ChunkTarget; nothing passes ChunkMax, even one very long line.
func TestPack(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString(strings.Repeat("word ", 40) + "\n\n") // 200 bytes a paragraph
	}
	b.WriteString(strings.Repeat("x", 9000) + "\n")
	got := split(KindText, b.String())
	if len(got) < 6 {
		t.Fatalf("%d chunks", len(got))
	}
	for i, p := range got {
		if len(p.Text) > ChunkMax {
			t.Fatalf("chunk %d is %d bytes", i, len(p.Text))
		}
		if p.Start < 1 || p.End < p.Start {
			t.Fatalf("chunk %d lines %d-%d", i, p.Start, p.End)
		}
	}
	if got[0].Start != 1 || len(got[0].Text) > ChunkTarget {
		t.Fatalf("first: %d-%d %d bytes", got[0].Start, got[0].End, len(got[0].Text))
	}
	last := got[len(got)-1]
	if last.Start != 101 || !strings.HasPrefix(last.Text, "x") {
		t.Fatalf("the long line's last piece: %+v", last.Start)
	}
}
