package extkit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// pb_cache: the standard folder for files an extension can rebuild.
//
//   - Where: pb_cache/ sits next to pb_data (beside pb_hooks, pb_public, ...); each extension owns
//     pb_cache/<ext-name>/. Extensions get the path from [CacheDir] or [HostCacheDir], never build it.
//   - Disposable: anything in it may be deleted at any time (`pocketone cache clear`, an admin, a cleanup), and the
//     extension carries on: it rebuilds, re-downloads or re-reads what it needs.
//   - Never backed up (it's outside pb_data) and never deployed or synced by ext/deploy; projects gitignore it.
//   - Per app: on a multisite host each instance has its own pb_cache/ in its folder, so it counts against that
//     instance's disk and one instance never sees another's. A cache that serves every instance of a host at once
//     uses [HostCacheDir].

// CacheFolder is the name of the cache folder next to pb_data.
const CacheFolder = "pb_cache"

// CacheRoot is app's pb_cache folder (not created).
func CacheRoot(app core.App) string {
	return filepath.Join(filepath.Dir(filepath.Clean(app.DataDir())), CacheFolder)
}

// CacheDir returns app's pb_cache/<ext>, creating it.
func CacheDir(app core.App, ext string) (string, error) {
	return makeCacheDir(CacheRoot(app), ext)
}

// HostCacheDir returns the folder for a cache ext shares across every app of the process: the host's
// pb_cache/<ext> when app is an instance of a multisite host that set one ([Scope.SetCacheRoot]), otherwise app's own
// (a single app is its own host). It creates the folder.
func HostCacheDir(app core.App, ext string) (string, error) {
	s := scopeOf(app)
	s.mu.Lock()
	host := s.host
	s.mu.Unlock()
	if host != nil {
		host.mu.Lock()
		root := host.cacheRoot
		host.mu.Unlock()
		if root != "" {
			return makeCacheDir(root, ext)
		}
	}
	return CacheDir(app, ext)
}

// CacheRoot is the pb_cache folder set with [Scope.SetCacheRoot] ("" when none).
func (s *Scope) CacheRoot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cacheRoot
}

// SetCacheRoot sets a host scope's own pb_cache folder, where [HostCacheDir] puts its instances' shared caches.
func (s *Scope) SetCacheRoot(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheRoot = dir
}

func makeCacheDir(root, ext string) (string, error) {
	if ext == "" || strings.ContainsAny(ext, `/\`) || ext == "." || ext == ".." {
		return "", fmt.Errorf("extkit: cache folder: bad extension name %q", ext)
	}
	dir := filepath.Join(root, ext)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// CacheUsage is one extension's folder in pb_cache and how much it holds.
type CacheUsage struct {
	Extension string
	Dir       string
	Bytes     int64
	Files     int
}

// CacheUsages lists every folder in root (a pb_cache folder) with its size, by name.
func CacheUsages(root string) ([]CacheUsage, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []CacheUsage
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		u := CacheUsage{Extension: e.Name(), Dir: filepath.Join(root, e.Name())}
		_ = filepath.WalkDir(u.Dir, func(_ string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, err := d.Info(); err == nil {
				u.Bytes += info.Size()
				u.Files++
			}
			return nil
		})
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Extension < out[j].Extension })
	return out, nil
}

// ClearCache empties root's folder for each of exts (all of them when exts is empty), keeping the folders. It returns
// the extensions it cleared.
func ClearCache(root string, exts ...string) ([]string, error) {
	usages, err := CacheUsages(root)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, e := range exts {
		want[e] = true
	}
	var cleared []string
	for _, u := range usages {
		if len(want) > 0 && !want[u.Extension] {
			continue
		}
		entries, err := os.ReadDir(u.Dir)
		if err != nil {
			return cleared, err
		}
		for _, e := range entries {
			if err := os.RemoveAll(filepath.Join(u.Dir, e.Name())); err != nil {
				return cleared, err
			}
		}
		cleared = append(cleared, u.Extension)
	}
	return cleared, nil
}
