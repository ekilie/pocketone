package pages

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"pocketone.dev/ext/extkit/js"
)

// require for page code (SPEC 1, 3.2): modules, JSX components and templates all resolve names the same way.
//
//   - "./x", "../x": a file in pb_pages, relative to the requiring file (never above pb_pages).
//   - "pkg", "pkg/sub", "@scope/pkg": first a _private folder, from the requiring file's folder up to the top of
//     pb_pages (the include lookup, SPEC 4.2); then the nearest node_modules folder, the same way up; a package
//     resolves through its package.json ("exports" or "main"), then index.js.
//   - anything not found there: the app's own module loader (pb_hooks paths; sandboxed under pocker).
//
// Go resolves; the driver (requireFrom in driver.js) loads.
//
// Every pb_pages lookup goes through the request's os.Root, so a name can't reach outside pb_pages.

// fileExts are tried, in order, after a path without a matching file.
var fileExts = []string{".js", ".ts", ".tsx", ".jsx", ".json", ".cjs"}

// resolveRequire resolves name for the pb_pages file from, for the driver's requireFrom: { file } for a pb_pages
// module, or { global, notFound } for the app's own module loader (notFound: the error to give if it doesn't have
// global either; unset when global is a path that was found).
func (r *request) resolveRequire(from, name string) (js.Object, error) {
	o := r.rt.NewObject()
	if name == "" {
		return nil, fmt.Errorf("%s: require(): empty name", from)
	}
	dir := path.Dir(from)
	if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") || name == "." || name == ".." {
		target := path.Clean(path.Join(dir, name))
		if target == ".." || strings.HasPrefix(target, "../") {
			return nil, fmt.Errorf("%s: require(%q) leaves pb_pages; for pb_hooks code use require(__hooks + '/...')", from, name)
		}
		if file, ok := r.resolvePath(target); ok {
			return o, o.Set("file", file)
		}
		return nil, fmt.Errorf("%s: require(%q): not found", from, name)
	}
	if file, ok := r.resolveBare(from, name); ok {
		return o, o.Set("file", file)
	}
	// Outside pb_pages: pb_hooks/node_modules, then the app folder's node_modules (where 0.x sites keep
	// package.json). The app's loader starts from __hooks, so it wouldn't walk up to these on its own.
	searched := []string{from + ": _private and node_modules folders up to pb_pages"}
	if pkg, _ := splitPackageName(name); pkg != "" && !strings.HasPrefix(name, "/") && !hasDotDotSegment(name) {
		for _, dir := range r.outsideModuleDirs() {
			searched = append(searched, dir)
			if st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(pkg))); err == nil && st.IsDir() {
				return o, o.Set("global", filepath.Join(dir, filepath.FromSlash(name)))
			}
		}
	}
	notFound := fmt.Sprintf("require(%q): not found; looked in %s", name, strings.Join(searched, ", "))
	return o, errors.Join(o.Set("global", name), o.Set("notFound", notFound))
}

// outsideModuleDirs are the node_modules folders a bare require tries after pb_pages: pb_hooks', then the app
// folder's (pb_pages' parent).
func (r *request) outsideModuleDirs() []string {
	var dirs []string
	if h := r.rt.Get("__hooks"); !h.Nullish() && h.String() != "" {
		dirs = append(dirs, filepath.Join(h.String(), "node_modules"))
	}
	if abs, err := filepath.Abs(r.p.dir); err == nil {
		d := filepath.Join(filepath.Dir(abs), "node_modules")
		if len(dirs) == 0 || dirs[0] != d {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// resolveBare finds a bare name in pb_pages: a _private folder from from's folder up, then node_modules from the same
// folder up. It doesn't try the app's own module loader.
func (r *request) resolveBare(from, name string) (string, bool) {
	if strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || hasDotDotSegment(name) {
		return "", false
	}
	if file, err := r.lookupPrivate(from, name, fileExts); err == nil {
		return file, true
	}
	return r.resolvePackage(path.Dir(from), name)
}

func hasDotDotSegment(name string) bool {
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." || seg == "." {
			return true
		}
	}
	return false
}

func (r *request) isFile(p string) bool {
	st, err := r.root.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func (r *request) isDir(p string) bool {
	st, err := r.root.Stat(p)
	return err == nil && st.IsDir()
}

// resolvePath finds a file for a pb_pages path: itself, with an extension, or a folder's package.json/index.
func (r *request) resolvePath(target string) (string, bool) {
	if r.isFile(target) {
		return target, true
	}
	for _, e := range fileExts {
		if r.isFile(target + e) {
			return target + e, true
		}
	}
	if r.isDir(target) {
		if main, ok, _ := r.packageEntry(target, "."); ok {
			return main, true
		}
		for _, e := range fileExts {
			if f := target + "/index" + e; r.isFile(f) {
				return f, true
			}
		}
	}
	return "", false
}

// resolvePackage looks for name in node_modules folders from dir up to the top of pb_pages.
func (r *request) resolvePackage(dir, name string) (string, bool) {
	pkg, sub := splitPackageName(name)
	if pkg == "" {
		return "", false
	}
	for {
		base := path.Join(dir, "node_modules", pkg)
		if r.isDir(base) {
			if sub == "" {
				if f, ok := r.resolvePath(base); ok {
					return f, true
				}
			} else {
				f, ok, hasExports := r.packageEntry(base, "./"+sub)
				if ok {
					return f, true
				}
				if hasExports {
					return "", false // "exports" lists everything a package makes public
				}
				if f, ok := r.resolvePath(path.Join(base, sub)); ok {
					return f, true
				}
			}
		}
		if dir == "." || dir == "" {
			return "", false
		}
		dir = path.Dir(dir)
	}
}

// splitPackageName splits "pkg/sub/path" or "@scope/pkg/sub" into the package and the subpath.
func splitPackageName(name string) (pkg, sub string) {
	parts := strings.Split(name, "/")
	n := 1
	if strings.HasPrefix(name, "@") {
		n = 2
	}
	if len(parts) < n || parts[n-1] == "" {
		return "", ""
	}
	return strings.Join(parts[:n], "/"), strings.Join(parts[n:], "/")
}

// packageEntry reads base/package.json and returns the file for subpath ("." for the package itself) from its
// "exports" (a string, or a map with "require"/"default"/"node" conditions) or, for ".", its "main". hasExports
// reports whether the package has an "exports" field, which makes anything it doesn't list private.
func (r *request) packageEntry(base, subpath string) (file string, ok, hasExports bool) {
	b, err := r.root.ReadFile(path.Join(base, "package.json"))
	if err != nil {
		return "", false, false
	}
	var pj struct {
		Main    string          `json:"main"`
		Exports json.RawMessage `json:"exports"`
	}
	if json.Unmarshal(b, &pj) != nil {
		return "", false, false
	}
	if len(pj.Exports) > 0 {
		if target, ok := exportTarget(pj.Exports, subpath); ok {
			f, ok := r.inPackage(base, target)
			return f, ok, true
		}
		return "", false, true
	}
	if subpath == "." && pj.Main != "" {
		f, ok := r.inPackage(base, pj.Main)
		return f, ok, false
	}
	return "", false, false
}

func (r *request) inPackage(base, target string) (string, bool) {
	p := path.Clean(path.Join(base, target))
	if p != base && !strings.HasPrefix(p, base+"/") {
		return "", false // a package can't point outside itself
	}
	return r.resolvePath(p)
}

// exportTarget picks the file for subpath from a package.json "exports" value.
func exportTarget(raw json.RawMessage, subpath string) (string, bool) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, subpath == "."
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return "", false
	}
	subpathKeys := false
	for k := range m {
		if strings.HasPrefix(k, ".") {
			subpathKeys = true
			break
		}
	}
	if !subpathKeys { // conditions only: they apply to "."
		if subpath != "." {
			return "", false
		}
		return condition(raw)
	}
	if v, ok := m[subpath]; ok {
		return condition(v)
	}
	return "", false
}

// condition resolves a conditional export value to a path, preferring require, then node, then default.
func condition(raw json.RawMessage) (string, bool) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, s != ""
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return "", false
	}
	for _, k := range []string{"require", "node", "default"} {
		if v, ok := m[k]; ok {
			if p, ok := condition(v); ok {
				return p, true
			}
		}
	}
	return "", false
}
