package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextVersion(t *testing.T) {
	for _, c := range []struct {
		from     string
		breaking bool
		want     string
	}{{"0.0.1", false, "0.0.2"}, {"0.0.1", true, "0.1.0"}, {"1.4.9", true, "1.5.0"}, {"0.0.0", false, "0.0.1"}} {
		if got, _ := nextVersion(c.from, c.breaking); got != c.want {
			t.Errorf("nextVersion(%s, %v) = %s, want %s", c.from, c.breaking, got, c.want)
		}
	}
}

func TestGithubAnchor(t *testing.T) {
	if got := githubAnchor("0.0.2 (2026-11-17)"); got != "002-2026-11-17" {
		t.Fatal(got)
	}
}

func TestChangelogSection(t *testing.T) {
	src := "# C\n\n## Unreleased\n\n## v1.0.0 (2026-01-01)\n\n### Added\n\n- a\n\n## v0.9.0 (2025-12-01)\n\n- b\n"
	got, ok := changelogSection(src, "v1.0.0")
	if !ok || !strings.HasPrefix(got, "### Added") || strings.Contains(got, "- b") {
		t.Fatalf("got %q %v", got, ok)
	}
	if _, ok := changelogSection(src, "Unreleased"); ok {
		t.Fatal("an empty section isn't notes")
	}
}

func TestAbsoluteLinks(t *testing.T) {
	got := absoluteLinks("- [Pages](ext/pages/CHANGELOG.md#002-x) and [up](CHANGELOG.pocketbase.md) and [web](https://x.dev/a.md)", "https://g/blob/v1/")
	want := "- [Pages](https://g/blob/v1/ext/pages/CHANGELOG.md#002-x) and [up](https://g/blob/v1/CHANGELOG.pocketbase.md) and [web](https://x.dev/a.md)"
	if got != want {
		t.Fatalf("got %s", got)
	}
}

// release raises each extension with unreleased lines, stamps its changelog and the root's, and lists the new
// versions in the root section; an extension with nothing unreleased is left alone.
func TestRelease(t *testing.T) {
	dir := t.TempDir()
	write := func(p, s string) {
		t.Helper()
		os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("CHANGELOG.md", "# PocketOne changelog\n\n## Unreleased\n\n### Added\n\n- core thing\n\n### PocketBase\n\n- Built on v1\n\n## v0.0.1 (2026-01-01)\n\n- old\n")
	write("ext/pages/README.md", "---\nowners: [\"@a\"]\nversion: 0.0.1\ntitle: \"Pages\"\n---\n# Pages\n")
	write("ext/pages/CHANGELOG.md", "# Pages changelog\n\n## Unreleased\n\n### Upgrading\n\n- rename x\n\n## PocketOne v0.0.1 (2026-01-01)\n\n- old\n")
	write("ext/pitr/README.md", "---\nversion: 0.0.3\ntitle: PITR\n---\n# PITR\n")
	write("ext/pitr/CHANGELOG.md", "# PITR changelog\n\n## Unreleased\n\n### Fixed\n\n- a fix\n")
	write("ext/quiet/README.md", "---\nversion: 0.2.0\ntitle: Quiet\n---\n# Quiet\n")
	write("ext/quiet/CHANGELOG.md", "# Quiet changelog\n\n## Unreleased\n")
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if _, err := release("v0.0.2", "2026-02-02"); err != nil {
		t.Fatal(err)
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }
	if got := read("ext/pages/README.md"); !strings.Contains(got, "version: 0.1.0\n") {
		t.Errorf("pages has upgrade steps, so minor: %s", got)
	}
	if got := read("ext/pitr/README.md"); !strings.Contains(got, "version: 0.0.4\n") {
		t.Errorf("pitr, patch: %s", got)
	}
	if got := read("ext/quiet/README.md"); !strings.Contains(got, "version: 0.2.0\n") {
		t.Errorf("quiet had nothing: %s", got)
	}
	if want := "## Unreleased\n\n## 0.1.0 (2026-02-02)\n\n### Upgrading\n\n- rename x\n\n## PocketOne v0.0.1"; !strings.Contains(read("ext/pages/CHANGELOG.md"), want) {
		t.Errorf("pages changelog:\n%s", read("ext/pages/CHANGELOG.md"))
	}
	root := read("CHANGELOG.md")
	want := "## Unreleased\n\n## v0.0.2 (2026-02-02)\n\n### Added\n\n- core thing\n\n### Extensions\n\n" +
		"- [Pages](ext/pages/CHANGELOG.md#010-2026-02-02) 0.0.1 → 0.1.0 (has upgrade steps)\n" +
		"- [PITR](ext/pitr/CHANGELOG.md#004-2026-02-02) 0.0.3 → 0.0.4\n\n### PocketBase\n\n- Built on v1\n\n## v0.0.1"
	if !strings.Contains(root, want) {
		t.Errorf("root changelog:\n%s", root)
	}
	if _, err := release("v0.0.2", "2026-02-02"); err == nil {
		t.Error("releasing the same version twice")
	}
	if _, err := release("v0.0.3", "2026-02-03"); err == nil || !strings.Contains(err.Error(), "nothing to release") {
		t.Errorf("nothing unreleased: %v", err)
	}
}
