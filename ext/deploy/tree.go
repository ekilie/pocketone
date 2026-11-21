package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path"
	fsv1 "pocketone.dev/ext/contracts/instance/fs/v1"
	"sort"
	"strings"
)

// The tree summarizes an instance's code set (without node_modules) as hashes, so two-way sync can tell what changed
// on the server without downloading a manifest of every file: a folder's hash covers everything below it, and a client
// polls the root hash and walks down only into folders whose hash changed. File hashes come from the store's cache
// (keyed by size and mtime), so a request costs a directory walk, not a re-hash.

// TreeEntry is a file or folder in a [TreeNode].
type TreeEntry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir,omitempty"`
	Hash string `json:"hash"`
	Size int64  `json:"size,omitempty"`
}

// TreeNode is one folder: its hash and (unless only the hash was asked for) its entries.
type TreeNode struct {
	Path    string      `json:"path"`
	Hash    string      `json:"hash"`
	Exists  bool        `json:"exists"`
	Entries []TreeEntry `json:"entries,omitempty"`
	// Folders, with deep=1: every folder below this one, each with its entries (never nested further), so a client
	// gets the whole tree in one request. Nil when deep wasn't asked (or an older server ignored it).
	Folders []TreeNode `json:"folders,omitempty"`
}

type treeDir struct {
	files map[string]currentFile
	dirs  map[string]bool
	hash  string
}

// buildTree groups files by folder and hashes each folder over its sorted entries ("d name hash" / "f name hash").
func buildTree(files map[string]currentFile) map[string]*treeDir {
	dirs := map[string]*treeDir{"": {files: map[string]currentFile{}, dirs: map[string]bool{}}}
	get := func(p string) *treeDir {
		d, ok := dirs[p]
		if !ok {
			d = &treeDir{files: map[string]currentFile{}, dirs: map[string]bool{}}
			dirs[p] = d
		}
		return d
	}
	for rel, f := range files {
		parent, name := path.Split(rel)
		parent = strings.TrimSuffix(parent, "/")
		get(parent).files[name] = f
		// register the folder chain up to the root
		for child := parent; child != ""; {
			up, cname := path.Split(child)
			up = strings.TrimSuffix(up, "/")
			get(up).dirs[cname] = true
			child = up
		}
	}
	// deepest folders first, so a folder's subfolders are hashed before it
	paths := make([]string, 0, len(dirs))
	for p := range dirs {
		paths = append(paths, p)
	}
	depth := func(p string) int {
		if p == "" {
			return 0
		}
		return strings.Count(p, "/") + 1
	}
	sort.Slice(paths, func(i, j int) bool { return depth(paths[i]) > depth(paths[j]) })
	for _, p := range paths {
		d := dirs[p]
		h := sha256.New()
		for _, e := range d.entries(p, dirs) {
			kind := "f"
			if e.Dir {
				kind = "d"
			}
			h.Write([]byte(kind + " " + e.Name + " " + e.Hash + "\n"))
		}
		d.hash = hex.EncodeToString(h.Sum(nil))
	}
	return dirs
}

func (d *treeDir) entries(p string, all map[string]*treeDir) []TreeEntry {
	out := make([]TreeEntry, 0, len(d.files)+len(d.dirs))
	for name := range d.dirs {
		out = append(out, TreeEntry{Name: name, Dir: true, Hash: all[path.Join(p, name)].hash})
	}
	for name, f := range d.files {
		out = append(out, TreeEntry{Name: name, Hash: f.hash, Size: f.size})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// GET /tree?path=<folder>[&hashOnly=1][&deep=1]: one folder of the code set (default the root) with its hash and
// entries; deep adds every folder below it (Folders), so a fresh client needs one request.
func (a *API) tree(w http.ResponseWriter, r *http.Request) error {
	c, err := a.authorize(r, false)
	if err != nil {
		return err
	}
	p := strings.Trim(r.URL.Query().Get("path"), "/")
	if p != "" && !fsv1.IsSafeRelativePath(p) {
		return errf(400, "Bad path")
	}
	files, _, err := c.store.scan(false)
	if err != nil {
		return err
	}
	dirs := buildTree(files)
	d, ok := dirs[p]
	node := TreeNode{Path: p, Exists: ok}
	if ok {
		node.Hash = d.hash
		if !flag(r, "hashOnly") {
			node.Entries = d.entries(p, dirs)
			if flag(r, "deep") {
				node.Folders = []TreeNode{}
				for dp, dd := range dirs {
					if dp != p && (p == "" || strings.HasPrefix(dp, p+"/")) {
						node.Folders = append(node.Folders, TreeNode{Path: dp, Hash: dd.hash, Exists: true, Entries: dd.entries(dp, dirs)})
					}
				}
				sort.Slice(node.Folders, func(i, j int) bool { return node.Folders[i].Path < node.Folders[j].Path })
			}
		}
	}
	writeJSON(w, 200, node)
	return nil
}
