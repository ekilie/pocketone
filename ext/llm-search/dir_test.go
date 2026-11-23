package llmsearch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	searchv1 "pocketone.dev/ext/contracts/llm/search/v1"
	"pocketone.dev/ext/extkit"
)

// siteFolder makes a folder of files beside the test app's pb_data (in the app's folder) and returns its path
// relative to that folder.
func siteFolder(t *testing.T, dataDir string, files map[string]string) (rel, full string) {
	t.Helper()
	root := filepath.Dir(dataDir)
	full, err := os.MkdirTemp(root, "site_")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(full) })
	for p, text := range files {
		f := filepath.Join(full, filepath.FromSlash(p))
		_ = os.MkdirAll(filepath.Dir(f), 0o755)
		if err := os.WriteFile(f, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rel, _ = filepath.Rel(root, full)
	return filepath.ToSlash(rel), full
}

const install = `---
title: Install
---

# Install

Intro paragraph about installing.

## macOS

Use brew to install the lighthouse keeper on a Mac.

## Setup

First setup notes about the fog horn.

## Setup

Second setup notes about the beacon lens.
`

// A server folder is every text file it keeps (dot files, node_modules and binaries left out), each result links to
// the page its file is published at with its section's anchor, and reading it again picks up changed and removed
// files.
func TestAddFolderOnServer(t *testing.T) {
	app, _, s := setup(t)
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "docs"})
	rel, full := siteFolder(t, app.DataDir(), map[string]string{
		"index.md":          "# Home\n\nWelcome to the lighthouse docs.\n",
		"guides/install.md": install,
		"notes.txt":         "Plain notes about the tide tables.\n",
		"main.go":           "package main // not included\n",
		".hidden.md":        "# Hidden\n\nsecret\n",
		"node_modules/x.md": "# Dep\n\ndependency\n",
		"img.md":            "\x00\x01binary",
	})
	src, err := s.AddFolder(ctx, extkit.Internal, searchv1.FolderInput{Bucket: "docs", Path: "./" + rel + "/",
		Include: []string{"*.md", "*.txt"}, URL: "/docs/{path}"})
	if err != nil || src.Kind != searchv1.KindDir || src.Name != rel || src.Path != rel || src.PageURL != "/docs/{path}" || src.Status != SourceReady {
		t.Fatalf("add folder: %+v %v", src, err)
	}
	drain(t, app)
	if got := docNames(t, s, "docs", rel); got != "guides/install.md,index.md,notes.txt" {
		t.Fatalf("documents: %s", got)
	}
	urls := map[string]string{} // first line of the passage -> its URL
	for _, q := range []string{"brew Mac", "beacon lens", "fog horn", "Welcome lighthouse docs", "tide tables"} {
		r := mustSearch(t, s, searchv1.SearchInput{Bucket: "docs", Query: q, Limit: 1})
		if len(r.Results) == 0 {
			t.Fatalf("%q: nothing", q)
		}
		urls[q] = r.Results[0].URL
	}
	want := map[string]string{"brew Mac": "/docs/guides/install#macos", "beacon lens": "/docs/guides/install#setup-1",
		"fog horn": "/docs/guides/install#setup", "Welcome lighthouse docs": "/docs", "tide tables": "/docs/notes"}
	for q, u := range want {
		if urls[q] != u {
			t.Errorf("%q: url %q, want %q", q, urls[q], u)
		}
	}

	// changed and removed files
	_ = os.Remove(filepath.Join(full, "notes.txt"))
	_ = os.WriteFile(filepath.Join(full, "guides", "install.md"), []byte(strings.Replace(install, "brew", "a package manager", 1)), 0o644)
	if _, err := s.Refresh(ctx, extkit.Internal, searchv1.SourceRef{Bucket: "docs", Source: rel}); err != nil {
		t.Fatal(err)
	}
	drain(t, app)
	if got := docNames(t, s, "docs", rel); got != "guides/install.md,index.md" {
		t.Fatalf("after changes: %s", got)
	}
	r := mustSearch(t, s, searchv1.SearchInput{Bucket: "docs", Query: "package manager", Limit: 1})
	if len(r.Results) == 0 || !strings.Contains(r.Results[0].Text, "package manager") {
		t.Fatalf("changed file: %+v", r)
	}

	// an unchanged folder is read again on its interval without work: its state stays
	svc := of(app)
	rec, _ := svc.findSource(mustBucket(t, svc, "docs").Id, rel)
	before := rec.GetString("state")
	if state, err := svc.readNow(mustBucket(t, svc, "docs"), rec, false); err != nil || state != before || state == "" {
		t.Fatalf("unchanged read: %q %v (was %q)", state, err, before)
	}
	if rec.GetDateTime("next").IsZero() {
		t.Fatal("a server folder isn't due to be read again")
	}
}

func mustBucket(t *testing.T, svc *service, name string) *core.Record {
	t.Helper()
	b, err := svc.findBucket(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A server folder stays inside the app's folder: no absolute paths, no .., no pb_data, no links out, and it must be
// a folder; a url pattern needs {path}.
func TestAddFolderOnServerChecks(t *testing.T) {
	app, _, s := setup(t)
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "docs"})
	rel, full := siteFolder(t, app.DataDir(), map[string]string{"a.md": "# A\n\ntext\n"})
	_ = os.Symlink("/usr", filepath.Join(full, "out"))         // out of the app's folder
	_ = os.Symlink(app.DataDir(), filepath.Join(full, "data")) // into pb_data
	dataRel, _ := filepath.Rel(filepath.Dir(app.DataDir()), app.DataDir())
	for _, p := range []string{"", "/etc", "..", "../x", rel + "/../..", dataRel, rel + "/a.md", rel + "/nope", rel + "/out", rel + "/data"} {
		if _, err := s.AddFolder(ctx, extkit.Internal, searchv1.FolderInput{Bucket: "docs", Path: p}); err == nil {
			t.Errorf("path %q was taken", p)
		}
	}
	if _, err := s.AddFolder(ctx, extkit.Internal, searchv1.FolderInput{Bucket: "docs", Path: rel, URL: "/docs"}); err == nil {
		t.Error("a url without {path} was taken")
	}
}

func TestPageURL(t *testing.T) {
	for _, c := range []struct{ pattern, file, want string }{
		{"/docs/{path}", "guides/install.md", "/docs/guides/install"},
		{"/docs/{path}", "guides/index.md", "/docs/guides"},
		{"/docs/{path}", "index.md", "/docs"},
		{"/{path}", "index.md", "/"},
		{"https://example.com/{path}.html", "a/b.md", "https://example.com/a/b.html"},
		{"", "a.md", ""},
	} {
		if got := pageURL(c.pattern, c.file); got != c.want {
			t.Errorf("pageURL(%q, %q) = %q, want %q", c.pattern, c.file, got, c.want)
		}
	}
}
