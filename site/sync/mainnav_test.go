package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMainNavDropsGoneExtensions: an extension's docs are linked at the end of their section (sync's own line, moved
// there from wherever it was), a link to an extension that no longer exists is dropped, and other hand-written links
// stay, an existing extension's page included.
func TestMainNavDropsGoneExtensions(t *testing.T) {
	root := t.TempDir()
	toc := `# New in PocketOne

- [Restore](/docs/features/restore)
- [AI in your app](/docs/extensions/agent)
- [AI in your app](/docs/extensions/llm)
- [Mail](/docs/extensions/mailtrap)

# Reference

- [CLI](/docs/reference/cli)
`
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(tocFile)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, tocFile), []byte(toc), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &manifest{Extensions: []*extension{
		{Name: "llm", Title: "LLM", landing: true, section: "New in PocketOne", docsTitle: "AI in your app"},
		{Name: "mailtrap", Title: "Mail trap"}, // a README page, no docs: a hand-written link to it stays
	}}
	out, err := mainNav(root, m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "/docs/extensions/agent") {
		t.Errorf("a removed extension's link survived:\n%s", out)
	}
	if strings.Count(out, "/docs/extensions/llm") != 1 || !strings.Contains(out, "- [Mail](/docs/extensions/mailtrap)\n- [AI in your app](/docs/extensions/llm)") {
		t.Errorf("llm's docs aren't linked once, at the end of their section:\n%s", out)
	}
	if !strings.Contains(out, "- [CLI](/docs/reference/cli)") || !strings.Contains(out, "- [Restore](/docs/features/restore)") {
		t.Errorf("hand-written links lost:\n%s", out)
	}
}
