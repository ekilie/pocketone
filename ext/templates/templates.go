// Package templates provides templates/v1: extensions' own pages as Go html/template sets that a project themes by
// ejecting a set into pb_templates/<set>/ and editing it. Any file there replaces the built-in one; the rest fall back.
package templates

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	templatesv1 "github.com/pockethost/pocketone/ext/contracts/templates/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's key: its pocketone.yaml switch and an instance's `extensions.<name>`.
const Name = "templates"

// Config sets where projects keep their copies.
type Config struct {
	// Dir is the folder holding one folder per ejected set; "" = pb_templates next to the app's pb_data.
	Dir string
}

// Extension provides templates/v1.
type Extension struct{ cfg Config }

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension.
func New(cfg Config) *Extension { return &Extension{cfg: cfg} }

// Name implements [extkit.Extension].
func (x *Extension) Name() string { return Name }

// Contracts implements [extkit.Declarer].
func (x *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("templates/v1")}
}

// Register implements [extkit.Extension]: it provides templates/v1 and adds `pocketone templates`.
func (x *Extension) Register(app core.App, _ extkit.Config) error {
	p := &provider{app: app, dir: x.cfg.Dir}
	templatesv1.Contract.Provide(app, p)
	if pb, ok := app.(*pocketbase.PocketBase); ok {
		pb.RootCmd.AddCommand(NewCommand(p))
	}
	return nil
}

type provider struct {
	app core.App
	dir string
}

var _ templatesv1.Templates = (*provider)(nil)

// Dir is the folder holding the project's copies (pb_templates).
func (p *provider) Dir() string {
	if p.dir != "" {
		return p.dir
	}
	return filepath.Join(p.app.DataDir(), "..", "pb_templates")
}

// Render implements templatesv1.Templates. The set is parsed on every render, so edits show on the next request.
func (p *provider) Render(e *core.RequestEvent, name, page string, status int, data any) error {
	base, ok := templatesv1.Set(p.app, name)
	if !ok {
		return fmt.Errorf("templates: no set %q", name)
	}
	var overlay fs.FS
	if st, err := os.Stat(filepath.Join(p.Dir(), name)); err == nil && st.IsDir() {
		overlay = os.DirFS(filepath.Join(p.Dir(), name))
	}
	t, err := templatesv1.Parse(base, overlay)
	if err != nil {
		return err
	}
	return templatesv1.Write(e, t, page, status, data)
}

// Eject implements templatesv1.Templates: a recursive copy of the set into pb_templates/<name>/ that skips identical
// files and overwrites changed ones only with force.
func (p *provider) Eject(name string, force bool) ([]string, error) {
	st, err := p.status(name)
	if err != nil {
		return nil, err
	}
	if len(st.changed) > 0 && !force {
		return nil, &templatesv1.ExistsError{Files: st.changed}
	}
	return st.write(append(st.missing, st.changed...))
}

// setStatus compares a set with the project's copy: the copy's paths of the files it lacks, of the ones it has
// unchanged, and of the ones it changed.
type setStatus struct {
	base                   fs.FS
	dir                    string
	missing, same, changed []string
}

func (p *provider) status(name string) (setStatus, error) {
	base, ok := templatesv1.Set(p.app, name)
	if !ok {
		return setStatus{}, fmt.Errorf("templates: no set %q (sets: %v)", name, templatesv1.Names(p.app))
	}
	st := setStatus{base: base, dir: filepath.Join(p.Dir(), name)}
	err := fs.WalkDir(base, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		want, err := fs.ReadFile(base, path)
		if err != nil {
			return err
		}
		target := filepath.Join(st.dir, filepath.FromSlash(path))
		have, err := os.ReadFile(target)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			st.missing = append(st.missing, target)
		case err != nil:
			return err
		case bytes.Equal(have, want):
			st.same = append(st.same, target)
		default:
			st.changed = append(st.changed, target)
		}
		return nil
	})
	return st, err
}

// NewCommand is `pocketone template`: list extensions' template sets, and eject them into pb_templates to edit.
func NewCommand(p *provider) *cobra.Command {
	cmd := &cobra.Command{Use: "template", Aliases: []string{"templates"}, Short: "Lists and ejects extensions' page templates (pb_templates)"}
	var force bool
	eject := &cobra.Command{
		Use:   "eject [set]",
		Short: "Copies a template set (every set without a name) into pb_templates/<set>/ for you to edit",
		Long: "Copies the set's built-in Go templates into pb_templates/<set>/; the app renders your copy of each file " +
			"from then on (delete a file to go back to the built-in one). Files you already have and haven't changed are " +
			"left alone; files you changed are only overwritten after you confirm, or with --force.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			names := templatesv1.Names(p.app)
			if len(args) == 1 {
				names = args
			}
			out := c.OutOrStdout()
			for _, n := range names {
				written, err := p.Eject(n, force)
				var ee *templatesv1.ExistsError
				if errors.As(err, &ee) {
					fmt.Fprintf(out, "%s: you changed %d file(s):\n", n, len(ee.Files))
					for _, f := range ee.Files {
						fmt.Fprintln(out, "  "+f)
					}
					if !isTerminal(c.InOrStdin()) {
						return fmt.Errorf("%s: not overwriting your changes; pass --force to replace them with the built-in files", n)
					}
					if !confirm(c, "Overwrite them with the built-in files?") {
						fmt.Fprintf(out, "%s: kept your changes\n", n)
						written, err = p.ejectMissing(n)
					} else {
						written, err = p.Eject(n, true)
					}
				}
				if err != nil {
					return err
				}
				for _, f := range written {
					fmt.Fprintln(out, "wrote "+f)
				}
				fmt.Fprintf(out, "%s: %d file(s) written to %s\n", n, len(written), filepath.Join(p.Dir(), n))
			}
			return nil
		},
	}
	eject.Flags().BoolVar(&force, "force", false, "overwrite files you changed without asking")
	list := &cobra.Command{
		Use:   "list",
		Short: "Lists the template sets extensions offer, and what you've changed in pb_templates",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			out := c.OutOrStdout()
			for _, n := range templatesv1.Names(p.app) {
				st, err := p.status(n)
				if err != nil {
					return err
				}
				if len(st.same)+len(st.changed) == 0 {
					fmt.Fprintf(out, "  %-12s built in (%d files)\n", n, len(st.missing))
				} else {
					fmt.Fprintf(out, "  %-12s ejected to %s: %d changed, %d unchanged, %d missing (built in)\n",
						n, st.dir, len(st.changed), len(st.same), len(st.missing))
				}
			}
			return nil
		},
	}
	cmd.AddCommand(list, eject)
	return cmd
}

// ejectMissing writes only the files the project's copy lacks, keeping its changes.
func (p *provider) ejectMissing(name string) ([]string, error) {
	st, err := p.status(name)
	if err != nil {
		return nil, err
	}
	return st.write(st.missing)
}

// write copies the built-in version of each of the copy's paths into place.
func (st setStatus) write(paths []string) ([]string, error) {
	var written []string
	for _, f := range paths {
		rel, err := filepath.Rel(st.dir, f)
		if err != nil {
			return written, err
		}
		b, err := fs.ReadFile(st.base, filepath.ToSlash(rel))
		if err != nil {
			return written, err
		}
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(f, b, 0o644); err != nil {
			return written, err
		}
		written = append(written, f)
	}
	sort.Strings(written)
	return written, nil
}

// isTerminal reports whether r is an interactive terminal (a prompt can be answered; /dev/null and pipes can't).
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// confirm asks a yes/no question on the command's input; anything but y/yes is no.
func confirm(c *cobra.Command, question string) bool {
	fmt.Fprintf(c.ErrOrStderr(), "%s (y/N) ", question)
	line, _ := bufio.NewReader(c.InOrStdin()).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
