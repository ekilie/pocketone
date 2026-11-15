package main

import "testing"

// The anchors must match the ids goldmark gives headings (ext/pages renders with parser.WithAutoHeadingID); these
// cases come from real pocketone.dev headings.
func TestHeadingID(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range []struct{ text, want string }{
		{"Homebrew (macOS and Linux)", "homebrew-macos-and-linux"},
		{"rate_limits:", "rate_limits"},
		{"Settings → Mail", "settings-mail"},
		{"Add your own tools from pb_hooks", "add-your-own-tools-from-pb_hooks"},
		{"1. The pb_pages folder", "1-the-pb_pages-folder"},
		{"GET /_pocker/whoami", "get-_pockerwhoami"},
		{"How it's built", "how-its-built"},
		{"Install", "install"},
		{"Install", "install-1"},
	} {
		if got := headingID(c.text, seen); got != c.want {
			t.Errorf("headingID(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestPageURL(t *testing.T) {
	for p, want := range map[string]string{
		"site/pb_pages/(main)/docs/installation.md":         "/docs/installation",
		"site/pb_pages/(main)/docs/plugins/deploy/index.md": "/docs/plugins/deploy",
		"site/pb_pages/(main)/docs/plugins/pages/spec.md":   "/docs/plugins/pages/spec",
		"site/pb_pages/(main)/docs/overview.ejs":            "/docs/overview",
		"site/pb_pages/(main)/blog/hello-world.md":          "/blog/hello-world",
	} {
		if got := pageURL(p); got != want {
			t.Errorf("pageURL(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestPageSections(t *testing.T) {
	md := "---\ntitle: \"Deploy\"\n---\n<!-- generated -->\n\n# Deploy\n\nIntro with [a link](/x) and `code`.\n\n" +
		"## Install\n\n- one\n- two\n\n```bash\n# not a heading\npocketone deploy\n```\n\n" +
		"| Key | Type |\n|---|---|\n| `dir` | string |\n\n### `deploy:`\n\nText.\n"
	got := pageSections("site/pb_pages/(main)/docs/plugins/deploy/index.md", md)
	want := []searchEntry{
		{URL: "/docs/plugins/deploy", Title: "Deploy", Text: "Intro with a link and code ."},
		{URL: "/docs/plugins/deploy#install", Title: "Deploy", Heading: "Install", Text: "one two # not a heading pocketone deploy dir string"},
		// the H1 "Deploy" already took "deploy"
		{URL: "/docs/plugins/deploy#deploy-1", Title: "Deploy", Heading: "deploy:", Text: "Text."},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d sections, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section %d:\n got  %+v\n want %+v", i, got[i], want[i])
		}
	}
}
