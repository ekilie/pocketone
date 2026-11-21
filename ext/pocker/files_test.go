package pocker

import (
	"path/filepath"
	"testing"

	"pocketone.dev/ext/extkit"
)

// An instance's cache is in the host's pb_cache (instances/<id>) when the host set one, else in its own folder.
func TestHostedCacheDir(t *testing.T) {
	root := t.TempDir()
	dirOf := func(id string) (string, bool) { return filepath.Join(root, id), true }
	host := extkit.NewScope("multisite")
	host.SetCacheRoot(filepath.Join(root, "pb_cache"))
	if d := hostedCacheDir(host, "abc", dirOf, "deploy"); d != filepath.Join(root, "pb_cache", "instances", "abc", "deploy") {
		t.Fatalf("hoisted: %s", d)
	}
	if d := hostedCacheDir(extkit.NewScope("multisite"), "abc", dirOf, "deploy"); d != filepath.Join(root, "abc", "pb_cache", "deploy") {
		t.Fatalf("no host root: %s", d)
	}
	if d := hostedCacheDir(nil, "abc", dirOf, "deploy"); d != filepath.Join(root, "abc", "pb_cache", "deploy") {
		t.Fatalf("no host: %s", d)
	}
}
