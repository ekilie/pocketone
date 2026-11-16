// Package bundle serves an app's files from an fs.FS (for example files embedded in the binary) instead of folders on
// disk: JS hooks and migrations for the jsvm extension, and the public files PocketBase serves from pb_public.
//
//	jsvm.MustRegister(app, jsvm.Config{Extensions: []any{bundle.Hooks(hooksFS), bundle.Migrations(migrationsFS)}})
//	bundle.Public(app, publicFS, true)
//
// .pb.js files come only from hooksFS's root (HooksDir is ignored, nothing is written to disk, no watching) and
// migrations only from migrationsFS's root; list only what you bundle.
//
// require() under __hooks resolves inside a sealed virtual root, [Root], that is never looked up on disk: paths under
// it are served from hooksFS, a module's relative requires stay inside, and everything else (absolute disk paths, `..`
// escapes, node_modules) is "module not found". JS that reads files itself ($os.readFile(__hooks + ...)) gets "no such
// file": the root doesn't exist on disk.
package bundle

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
)

// Root is the value of __hooks with bundled hooks: a virtual root whose require() paths are served from the FS.
const Root = "/pb_embedded/pb_hooks"

// root is Root; tests point it at a real directory to prove the disk is ignored.
var root = Root

// Hooks is a jsvm extension serving the hook files from fsys's root, with require() under __hooks sealed to fsys.
func Hooks(fsys fs.FS) any { return hooks{fsys} }

// Migrations is a jsvm extension serving the migration files from fsys's root; MigrationsDir is then ignored.
func Migrations(fsys fs.FS) any { return migrations{fsys} }

// Public serves fsys at the app's root the way PocketBase serves pb_public: GET /{path...}, unless a route already
// claims it, bound last. With indexFallback, a missing path serves index.html (for single-page apps).
func Public(app core.App, fsys fs.FS, indexFallback bool) {
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(e *core.ServeEvent) error {
			if !e.Router.HasRoute(http.MethodGet, "/{path...}") {
				e.Router.GET("/{path...}", apis.Static(fsys, indexFallback))
			}
			return e.Next()
		},
		Priority: 999, // after the app's own routes
	})
}

type hooks struct{ fs fs.FS }

// HookFiles implements the jsvm extension method.
func (h hooks) HookFiles(pattern string) (map[string][]byte, error) {
	return filesContent(h.fs, pattern)
}

// Modules implements the jsvm extension method: require() is sealed to the FS under Root.
func (h hooks) Modules() (string, func(string) ([]byte, error)) {
	vroot := root
	return vroot, func(p string) ([]byte, error) {
		p = path.Clean(filepath.ToSlash(p))
		rel, ok := strings.CutPrefix(p, vroot+"/")
		if !ok || !fs.ValidPath(rel) || rel == "node_modules" || strings.HasPrefix(rel, "node_modules/") ||
			strings.Contains(rel, "/node_modules/") || strings.HasSuffix(rel, "/node_modules") {
			return nil, fs.ErrNotExist
		}
		info, err := fs.Stat(h.fs, rel)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			return nil, fs.ErrNotExist
		}
		return fs.ReadFile(h.fs, rel)
	}
}

type migrations struct{ fs fs.FS }

// MigrationFiles implements the jsvm extension method.
func (m migrations) MigrationFiles(pattern string) (map[string][]byte, error) {
	return filesContent(m.fs, pattern)
}

// filesContent returns the direct files at fsys's root matching pattern, with their content.
func filesContent(fsys fs.FS, pattern string) (map[string][]byte, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string][]byte{}, nil
		}
		return nil, err
	}
	var exp *regexp.Regexp
	if pattern != "" {
		if exp, err = regexp.Compile(pattern); err != nil {
			return nil, err
		}
	}
	result := map[string][]byte{}
	for _, e := range entries {
		if e.IsDir() || (exp != nil && !exp.MatchString(e.Name())) {
			continue
		}
		raw, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		result[e.Name()] = raw
	}
	return result, nil
}
