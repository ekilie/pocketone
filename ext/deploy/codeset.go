package deploy

import (
	"fmt"
	fsv1 "pocketone.dev/ext/contracts/instance/fs/v1"
	"regexp"
	"slices"
	"strings"
)

// The deploy "code set": the only files a deploy may write or delete inside an instance folder, instance.fs/v1's code
// paths (fsv1.IsCodePath). pb_data is never part of it. The root node_modules tree is (the CLI ships the production
// dependency tree); node_modules anywhere else stays out.

const maxManifestFiles = 20_000

var hashRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

func isSha256Hex(v string) bool { return hashRE.MatchString(v) }

func isNodeModulesPath(p string) bool { return strings.SplitN(p, "/", 2)[0] == fsv1.NodeModulesDir }

// Manifest maps a code-set path to its sha256 (lowercase hex).
type Manifest map[string]string

// ManifestError is a user-facing manifest problem (400).
type ManifestError struct{ msg string }

func (e *ManifestError) Error() string { return e.msg }

// validateManifest checks a client manifest { path: sha256 } (raw is the decoded JSON value).
func validateManifest(raw any) (Manifest, error) {
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, &ManifestError{"files must be an object of { path: sha256 }"}
	}
	if len(obj) > maxManifestFiles {
		nm := 0
		for p := range obj {
			if isNodeModulesPath(p) {
				nm++
			}
		}
		inNM, hint := "", ""
		if nm > 0 {
			inNM = fmt.Sprintf(", %d of them in node_modules", nm)
			hint = " Make sure only production dependencies are deployed."
		}
		return nil, &ManifestError{fmt.Sprintf("Too many files (%d%s); a deploy can have at most %d.%s", len(obj), inNM, maxManifestFiles, hint)}
	}
	out := make(Manifest, len(obj))
	// sorted for deterministic errors
	paths := make([]string, 0, len(obj))
	for p := range obj {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		if !fsv1.IsCodePath(p) {
			return nil, &ManifestError{p + " is outside the deployable set (pb_* except pb_data, package.json, bun.lock(b), patches/, " +
				"node_modules/ at the project root; no .git* files, no node_modules inside other folders)"}
		}
		h, _ := obj[p].(string)
		if !isSha256Hex(h) {
			return nil, &ManifestError{p + ": expected a lowercase sha256 hex digest"}
		}
		out[p] = h
	}
	// a path can't be both a file and a directory
	for _, p := range paths {
		segs := strings.Split(p, "/")
		for i := 1; i < len(segs); i++ {
			prefix := strings.Join(segs[:i], "/")
			if _, ok := out[prefix]; ok {
				return nil, &ManifestError{prefix + " is both a file and a directory"}
			}
		}
	}
	return out, nil
}
