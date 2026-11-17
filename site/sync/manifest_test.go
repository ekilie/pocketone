package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const testGroups = `{"groups": [{"id": "services", "title": "Services"}, {"id": "authors", "title": "Authors"}],
 "links": [{"name": "kit", "title": "Kit", "group": "authors", "summary": "The kit.", "link": "ext/kit/kit.go"}]}`

// An extension lists itself from its own README: nothing outside its folder names it.
func TestManifestFromFrontMatter(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"site/extensions.json":  testGroups,
		"ext/kit/kit.go":        "package kit\n",
		"ext/zeta/README.md":    "---\nowners: [\"@a\"]\nversion: 0.1.0\ntitle: \"Zeta\"\ngroup: services\nsummary: \"Does z: with a colon.\"\n---\n\n# Zeta\n",
		"ext/zeta/CHANGELOG.md": "# Zeta changelog\n\n## Unreleased\n",
		"ext/zeta/GUIDE.md":     "# A guide\n",
		"ext/zeta/TODO.md":      "# Zeta: blocked work\n",
		"ext/alpha/README.md":   "---\ntitle: Alpha\ngroup: services\nsummary: First.\n---\n# Alpha\n",
	})
	m, err := loadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range m.Extensions {
		names = append(names, p.Name)
	}
	if got := strings.Join(names, ","); got != "alpha,kit,zeta" {
		t.Fatalf("extensions, by title: got %s", got)
	}
	z := m.Extensions[2]
	if z.Title != "Zeta" || z.Group != "services" || z.Summary != "Does z: with a colon." {
		t.Fatalf("zeta: %+v", z)
	}
	if len(z.guides) != 1 || z.guides[0].slug != "guide" {
		t.Fatalf("zeta's guides (CHANGELOG.md and TODO.md aren't ones): %+v", z.guides)
	}
}

func TestManifestProblems(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"site/extensions.json":    testGroups,
		"ext/kit/kit.go":          "package kit\n",
		"ext/nosummary/README.md": "---\ntitle: X\ngroup: services\n---\n# X\n",
		"ext/badgroup/README.md":  "---\ntitle: Y\ngroup: nope\nsummary: y\n---\n# Y\n",
		"ext/noreadme/x.go":       "package x\n",
	})
	_, err := loadManifest(root)
	if err == nil {
		t.Fatal("want problems")
	}
	for _, want := range []string{"ext/nosummary/README.md: front matter needs `summary:`", `ext/badgroup: unknown group "nope"`, "ext/noreadme: no README.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%v", want, err)
		}
	}
}
