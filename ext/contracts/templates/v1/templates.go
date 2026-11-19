// Package templatesv1 is the templates/v1 contract: an extension's own pages as a set of Go html/template files that a
// project can override. An extension adds its set (usually go:embed) under a name; the provider renders a page from
// the set, taking the project's copy of any file it has, and ejects the set for the project to edit.
//
// A set is every *.tpl file in an fs.FS, at any depth. Each file is one template named by its path without ".tpl"
// ("signin", "parts/head"), so pages include pieces with {{template "parts/head" .}}.
package templatesv1

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"sync"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"pocketone.dev/ext/extkit"
)

// Templates renders and ejects the sets added with [Add].
type Templates interface {
	// Render writes page of set name as the whole response to e (status 0 = 200), with data as the template's dot.
	// Each template comes from the project's copy when it has one, else from the set.
	Render(e *core.RequestEvent, name, page string, status int, data any) error

	// Eject copies set name's files into the project's copy, for the project to edit. Files the copy lacks are written
	// and files identical to the built-in ones are left alone; files that differ (the project's edits) are only
	// overwritten with force: otherwise it refuses with an *ExistsError naming them, writing nothing. It returns the
	// files written.
	Eject(name string, force bool) (written []string, err error)
}

// Contract is templates/v1.
var Contract = extkit.NewContract[Templates]("templates/v1", extkit.One)

// ExistsError is Eject's refusal: the project's copies of these files differ from the built-in ones.
type ExistsError struct{ Files []string }

func (e *ExistsError) Error() string {
	return fmt.Sprintf("%d file(s) differ from the built-in ones (use --force to overwrite): %v", len(e.Files), e.Files)
}

const storeKey = "templates/v1.sets"

type registry struct {
	mu   sync.Mutex
	sets map[string]fs.FS
}

func registryOf(app core.App) *registry {
	return app.Store().GetOrSet(storeKey, func() any { return &registry{sets: map[string]fs.FS{}} }).(*registry)
}

// Add registers an extension's template set on app under name (call it in Register). name is a plain folder name
// ("oauth"): the project's copy lives in a folder of that name.
func Add(app core.App, name string, files fs.FS) {
	if !fs.ValidPath(name) || name == "." || strings.Contains(name, "/") {
		panic(fmt.Sprintf("templates/v1: bad set name %q (a plain folder name like \"oauth\")", name))
	}
	r := registryOf(app)
	r.mu.Lock()
	r.sets[name] = files
	r.mu.Unlock()
}

// Set returns the files added under name (for providers).
func Set(app core.App, name string) (fs.FS, bool) {
	r := registryOf(app)
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.sets[name]
	return f, ok
}

// Names lists the sets added on app, sorted (for providers).
func Names(app core.App) []string {
	r := registryOf(app)
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.sets))
	for n := range r.sets {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Parse builds a set's templates: every *.tpl in base, then every *.tpl in overlay (nil for none), which replaces the
// template of the same name. Providers and an extension rendering its own set without a provider both use it.
func Parse(base, overlay fs.FS) (*template.Template, error) {
	t := template.New("")
	for _, fsys := range []fs.FS{base, overlay} {
		if fsys == nil {
			continue
		}
		err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".tpl") {
				return err
			}
			b, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}
			if _, err := t.New(strings.TrimSuffix(p, ".tpl")).Parse(string(b)); err != nil {
				return fmt.Errorf("templates: %s: %w", p, err)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

// Write executes page of t with data and writes it as the whole HTML response to e (status 0 = 200). Nothing is
// written if the template fails.
func Write(e *core.RequestEvent, t *template.Template, page string, status int, data any) error {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, page, data); err != nil {
		return err
	}
	if status == 0 {
		status = http.StatusOK
	}
	return e.HTML(status, buf.String())
}
