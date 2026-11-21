package fsv1

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// An instance's code: the pb_* folders except pb_data, package.json and the bun lockfiles at the top, patches/, and
// the production node_modules tree at the top. pb_data is never part of it, and nothing in this package reaches it.

// RootCodeFiles are the code files at the top of an instance folder.
var RootCodeFiles = []string{"package.json", "bun.lock", "bun.lockb"}

const (
	DataDir        = "pb_data"
	NodeModulesDir = "node_modules"
	PatchesDir     = "patches"
	MaxPathLength  = 1024
)

// IsSafeRelativePath: a relative POSIX path with no empty, "." or ".." segments, no backslash or NUL.
func IsSafeRelativePath(p string) bool {
	if p == "" || len(p) > MaxPathLength {
		return false
	}
	if strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// IsGitSegment: .git, .gitignore, ... never belong to the code.
func IsGitSegment(seg string) bool { return strings.HasPrefix(seg, ".git") }

// IsCodeRoot: a top-level name the code lives under (or is).
func IsCodeRoot(name string) bool {
	return (strings.HasPrefix(name, "pb_") && name != DataDir) || name == PatchesDir || slices.Contains(RootCodeFiles, name)
}

// IsCodePath reports whether p (relative to the instance folder) belongs to the code: what a deploy may write or
// delete. node_modules counts only at the top.
func IsCodePath(p string) bool {
	if !IsSafeRelativePath(p) {
		return false
	}
	segs := strings.Split(p, "/")
	if slices.ContainsFunc(segs, IsGitSegment) {
		return false
	}
	first := segs[0]
	if first == NodeModulesDir {
		return len(segs) > 1
	}
	if slices.Contains(segs, NodeModulesDir) {
		return false
	}
	if len(segs) == 1 {
		return slices.Contains(RootCodeFiles, first) || (strings.HasPrefix(first, "pb_") && first != DataDir)
	}
	if first == DataDir {
		return false
	}
	return strings.HasPrefix(first, "pb_") || first == PatchesDir
}

// IsReachable reports whether a file API may read or write p: the code, node_modules at the top or inside a code
// folder, and patches/. Never pb_data or .git*.
func IsReachable(p string) bool {
	if !IsSafeRelativePath(p) {
		return false
	}
	segs := strings.Split(p, "/")
	first := segs[0]
	if first == DataDir || slices.ContainsFunc(segs, IsGitSegment) {
		return false
	}
	if first == NodeModulesDir || first == PatchesDir || IsCodePath(p) {
		return true
	}
	return len(segs) > 1 && strings.HasPrefix(first, "pb_") && slices.Contains(segs, NodeModulesDir)
}

// PathError is a client path Resolve refuses.
type PathError struct{ Path, Reason string }

func (e *PathError) Error() string { return e.Path + ": " + e.Reason }

// ErrUnreachable is the reason for a path outside what [IsReachable] allows.
var ErrUnreachable = errors.New("outside the files you can reach (pb_hooks, pb_migrations, pb_public and other pb_* folders except pb_data, node_modules, package.json, bun.lock(b), patches/)")

// Resolve maps a client path (slash-separated, relative to dir; "" is dir itself) to a file path inside dir. It
// refuses paths [IsReachable] doesn't allow, paths that cross a symbolic link, and anything that would land outside
// dir. rel is the cleaned path.
func Resolve(dir, raw string) (rel, fsPath string, err error) {
	p := strings.Trim(raw, "/")
	if p == "" {
		return "", dir, nil
	}
	if !IsReachable(p) {
		return "", "", &PathError{p, ErrUnreachable.Error()}
	}
	fsPath = filepath.Join(dir, filepath.FromSlash(p))
	cur := dir
	for _, seg := range strings.Split(p, "/") {
		cur = filepath.Join(cur, seg)
		st, err := os.Lstat(cur)
		if err != nil {
			break
		}
		if st.Mode()&fs.ModeSymlink != 0 {
			return "", "", &PathError{p, "crosses a symbolic link"}
		}
	}
	realRoot, err := filepath.EvalSymlinks(dir)
	if err != nil {
		realRoot = dir
	}
	if realParent, err := filepath.EvalSymlinks(filepath.Dir(fsPath)); err == nil {
		if r, err := filepath.Rel(realRoot, realParent); err != nil || r == ".." || strings.HasPrefix(r, "../") {
			return "", "", &PathError{p, "outside the instance folder"}
		}
	}
	return p, fsPath, nil
}
