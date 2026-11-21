package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	remoteclient "pocketone.dev/ext/contracts/remote/v1/remoteclient"
)

// CloneOptions configure [Clone].
type CloneOptions struct {
	URL  string // the remote app
	Dir  string // the new project folder (default: named after the URL's host)
	Name string // the remote's name (default "default")
	// Login signs in to the new remote (normally remoteclient.Login with the command's flags).
	Login func(ctx context.Context, c *remoteclient.Client) error
	Log   func(string)
}

// Clone starts a local project from a running app: it makes the folder (refusing one that isn't empty), adds the
// remote as the project's default, signs in, and pulls the app's code (pb_data never comes). It returns the folder
// and how many files it pulled. On an error, a folder Clone made is removed again unless the pull got some files.
func Clone(ctx context.Context, opts CloneOptions) (dir string, n int, err error) {
	log := opts.Log
	if log == nil {
		log = func(string) {}
	}
	u, err := remoteclient.NormalizeURL(opts.URL)
	if err != nil {
		return "", 0, err
	}
	dir = opts.Dir
	if dir == "" {
		dir = CloneDirName(u)
	}
	if dir, err = filepath.Abs(dir); err != nil {
		return "", 0, err
	}
	created, err := makeEmptyDir(dir)
	if err != nil {
		return "", 0, err
	}
	made := dir
	defer func() {
		if err != nil && created && n == 0 {
			_ = os.RemoveAll(made) // not dir: an error return has set it to ""
		}
	}()

	// a new project at dir itself, even inside another project
	name := opts.Name
	if name == "" {
		name = remoteclient.DefaultName
	}
	p := &remoteclient.Project{Root: dir, Remotes: map[string]*remoteclient.Remote{}}
	r, err := p.Add(name, u)
	if err != nil {
		return "", 0, err
	}
	p.Default = r.Name
	if err := p.Save(); err != nil {
		return "", 0, err
	}
	if opts.Login != nil {
		if err := opts.Login(ctx, &remoteclient.Client{Remote: r, HTTP: remoteclient.NewHTTPClient()}); err != nil {
			return "", 0, err
		}
	}

	rc, err := remoteclient.Open(dir, name)
	if err != nil {
		return "", 0, err
	}
	wctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	c, err := NewClient(wctx, rc)
	cancel()
	if err != nil {
		return "", 0, err
	}
	// keep what came down if the pull stops partway
	n, err = Pull(ctx, c, PullOptions{Dir: dir, Log: log})
	return dir, n, err
}

// CloneDirName is the default folder for a clone of u: the host's first label (pocketcon.dev → pocketcon,
// myapp.pockethost.io → myapp), or "app" for localhost and IP addresses.
func CloneDirName(u string) string {
	pu, err := url.Parse(u)
	if err != nil {
		return "app"
	}
	host := pu.Hostname()
	if host == "" || host == "localhost" || net.ParseIP(host) != nil {
		return "app"
	}
	return strings.SplitN(host, ".", 2)[0]
}

// makeEmptyDir makes dir, or accepts it when it exists and is empty. created says Clone made it.
func makeEmptyDir(dir string) (created bool, err error) {
	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return true, os.MkdirAll(dir, 0o755)
	case err != nil:
		return false, err
	case len(entries) > 0:
		return false, fmt.Errorf("%s already exists and isn't empty", dir)
	}
	return false, nil
}

func cloneCommand() *cobra.Command {
	var (
		name                         string
		token                        string
		tokenStdin, device, noBrowse bool
	)
	c := &cobra.Command{
		Use:   "clone <url> [dir]",
		Short: "Starts a local project from a running app: makes the folder, links and signs in, and pulls its code",
		Long: "Makes dir (default: named after the app's host, e.g. pocketcon for https://pocketcon.dev), adds the app " +
			"as the project's default remote, signs in (in the browser when the app offers it, else with an API token), " +
			"and pulls pb_hooks, pb_migrations, pb_pages, pb_public, pb_templates, package.json, the lockfile and " +
			"patches/. pb_data never comes. Then run `dev` in the folder to keep both sides in step.",
		Example: "clone https://pocketcon.dev\nclone https://myapp.pockethost.io myapp --token ph_pat_...",
		Args:    cobra.RangeArgs(1, 2), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			out := cmd.OutOrStdout()
			log := printer(out)
			opts := CloneOptions{URL: args[0], Name: name, Log: log}
			if len(args) == 2 {
				opts.Dir = args[1]
			}
			opts.Login = func(ctx context.Context, c *remoteclient.Client) error {
				lctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
				defer cancel()
				lo := remoteclient.LoginOptions{Token: token, Browser: remoteclient.OAuthLogin, Device: device, NoBrowser: noBrowse, Out: out}
				if tokenStdin {
					lo.TokenFrom = cmd.InOrStdin()
				}
				if isTerminal(cmd.InOrStdin()) {
					lo.Prompt = cmd.InOrStdin()
				}
				_, err := remoteclient.Login(lctx, c, lo)
				return err
			}
			dir, n, err := Clone(ctx, opts)
			if err != nil {
				return err
			}
			log(fmt.Sprintf("Pulled %d file(s) into %s.", n, dir))
			log("Next:")
			log("  cd " + relOrAbs(dir))
			if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
				log(fmt.Sprintf("  %-22s # or npm install: dev and deploy upload the production packages from here", "bun install"))
			}
			log(fmt.Sprintf("  %-22s # edits go up as you save; changes on the app come down", cmd.Root().Name()+" dev"))
			return nil
		},
	}
	c.Flags().StringVar(&name, "name", remoteclient.DefaultName, "remote name")
	c.Flags().StringVar(&token, "token", "", "use this API token (pbt_..., a PocketHost token, or a JWT)")
	c.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read the token from stdin")
	c.Flags().BoolVar(&device, "device", false, "sign in on another device with a code (SSH, headless)")
	c.Flags().BoolVar(&noBrowse, "no-browser", false, "print the sign-in URL instead of opening a browser")
	return c
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// relOrAbs is dir relative to the working directory when it's below it, else absolute.
func relOrAbs(dir string) string {
	wd, err := os.Getwd()
	if err != nil {
		return dir
	}
	if rel, err := filepath.Rel(wd, dir); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return dir
}
