package llmsearch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// DirEvery is how often a server folder is read again (on the five-minute refresh tick). An unchanged folder (the
// same files, sizes and modification times) costs a walk of it and nothing more.
const DirEvery = 5 * time.Minute

// MaxDirFiles is the most files a server folder source reads.
const MaxDirFiles = 5000

// appRoot is the app's own folder: the one holding pb_data (and pb_pages, pb_public, pb_hooks beside it).
func (s *service) appRoot() (string, error) {
	root, err := filepath.Abs(filepath.Dir(s.app.DataDir()))
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	return root, nil
}

// appFolder checks a server folder's path: relative to the app's folder, inside it after symlinks, not pb_data, and
// a folder. It returns the path cleaned (slashes, no leading ./) and in full.
func (s *service) appFolder(p string) (rel, full string, err error) {
	p = strings.TrimSpace(strings.ReplaceAll(p, `\`, "/"))
	if p == "" {
		return "", "", badRequest("give the folder's path, relative to the app's folder (\"pb_pages/docs\")")
	}
	if path.IsAbs(p) || filepath.IsAbs(p) {
		return "", "", badRequest("the folder's path is relative to the app's folder, not absolute")
	}
	rel = path.Clean(p)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", "", badRequest("the folder must be inside the app's folder")
	}
	root, err := s.appRoot()
	if err != nil {
		return "", "", err
	}
	full = filepath.Join(root, filepath.FromSlash(rel))
	real, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", "", badRequest(fmt.Sprintf("no folder %s in the app's folder", rel))
	}
	if !inside(root, real) {
		return "", "", badRequest("the folder must be inside the app's folder")
	}
	if data, err := filepath.EvalSymlinks(s.app.DataDir()); err == nil && inside(data, real) {
		return "", "", badRequest("pb_data can't be searched")
	}
	if st, err := os.Stat(real); err != nil || !st.IsDir() {
		return "", "", badRequest(fmt.Sprintf("%s isn't a folder", rel))
	}
	return rel, real, nil
}

// inside reports whether p is dir or below it.
func inside(dir, p string) bool {
	r, err := filepath.Rel(dir, p)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)
}

// included reports whether a file is kept by a folder's include patterns (none keeps everything).
func included(p string, include []string) bool {
	if len(include) == 0 {
		return true
	}
	for _, pat := range include {
		pat = strings.Trim(pat, "/")
		if ok, _ := path.Match(pat, p); ok {
			return true
		}
		if ok, _ := path.Match(pat, path.Base(p)); ok {
			return true
		}
	}
	return false
}

// readDir reads a server folder source: every text file it keeps becomes a document named by its path in the folder.
// The folder's listing (paths, sizes, modification times) is its state, so an unchanged folder reads nothing.
func (s *service) readDir(b, src *core.Record, force bool) (string, error) {
	sp := specOf(src)
	_, full, err := s.appFolder(sp.Path)
	if err != nil {
		return "", err
	}
	root, err := s.appRoot()
	if err != nil {
		return "", err
	}
	data, _ := filepath.EvalSymlinks(s.app.DataDir())
	type file struct {
		rel, full string
		size      int64
		mod       time.Time
	}
	var files []file
	err = filepath.WalkDir(full, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable: left out
		}
		if p == full {
			return nil
		}
		rel, _ := filepath.Rel(full, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if data != "" && p == data || strings.HasPrefix(d.Name(), ".") || skipDirs[d.Name()] || d.Name() == "pb_data" {
				return filepath.SkipDir
			}
			return nil
		}
		if skipPath(rel, sp.Exclude) || !included(rel, sp.Include) {
			return nil
		}
		real, err := filepath.EvalSymlinks(p)
		if err != nil || !inside(root, real) || (data != "" && inside(data, real)) {
			return nil // a link out of the app's folder, or into pb_data
		}
		info, err := os.Stat(real)
		if err != nil || !info.Mode().IsRegular() || info.Size() > MaxFileBytes {
			return nil
		}
		if len(files) >= MaxDirFiles {
			return fmt.Errorf("over %d files: narrow it with include or exclude", MaxDirFiles)
		}
		files = append(files, file{rel: rel, full: real, size: info.Size(), mod: info.ModTime()})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00%d\x00%d\n", f.rel, f.size, f.mod.UnixNano())
	}
	state := hex.EncodeToString(h.Sum(nil))
	if !force && state == src.GetString("state") {
		return state, nil
	}
	docs := make([]docIn, 0, len(files))
	for _, f := range files {
		raw, err := os.ReadFile(f.full)
		if err != nil || !isText(raw) {
			continue // binaries are left out
		}
		docs = append(docs, docIn{name: f.rel, text: string(raw)})
	}
	return state, s.syncDocs(b, src, docs, force)
}
