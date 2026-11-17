package main

import "testing"

func TestReleaseVersion(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"CHANGELOG.md": "# Changelog\n\n## Unreleased\n\n- next\n\n## v0.0.1-alpha.6 (2026-11-16)\n\n## v0.0.1-alpha.5 (2026-11-15)\n",
	})
	v, err := releaseVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	if v != "v0.0.1-alpha.6" {
		t.Fatalf("got %q, want v0.0.1-alpha.6", v)
	}

	writeFiles(t, root, map[string]string{"CHANGELOG.md": "# Changelog\n\n## Unreleased\n"})
	if _, err := releaseVersion(root); err == nil {
		t.Fatal("want an error with no release heading")
	}
}
