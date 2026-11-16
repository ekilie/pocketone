package deploy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	remoteclient "github.com/pockethost/pocketone/ext/contracts/remote/v1/remoteclient"
	"github.com/pockethost/pocketone/ext/deploy/internal/glob"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/spf13/cobra"
)

// Commands returns the client commands: deploy, dev, pull, logs.
func Commands() []*cobra.Command {
	return []*cobra.Command{deployCommand(), devCommand(), pullCommand(), logsCommand(), restartCommand()}
}

type setFlags struct {
	include       []string
	exclude       []string
	noNodeModules bool
	nodeModules   bool
	verbose       bool
}

func (f *setFlags) bind(c *cobra.Command) {
	c.Flags().StringSliceVarP(&f.include, "include", "i", nil, "add include patterns (default set: "+strings.Join(DefaultIncludes, ",")+")")
	c.Flags().StringSliceVarP(&f.exclude, "exclude", "e", nil, "exclude patterns (pb_data is always excluded)")
	c.Flags().BoolVar(&f.nodeModules, "node-modules", false, "ship the production node_modules tree (default with a package.json)")
	c.Flags().BoolVar(&f.noNodeModules, "no-node-modules", false, "don't ship node_modules; leave the remote's alone")
	c.Flags().BoolVarP(&f.verbose, "verbose", "v", false, "list every file")
}

func (f *setFlags) collect(dir string) CollectOptions {
	o := CollectOptions{Dir: dir, Exclude: f.exclude}
	if len(f.include) > 0 {
		o.Include = append(append([]string{}, DefaultIncludes...), f.include...)
	}
	switch {
	case f.noNodeModules:
		v := false
		o.NodeModules = &v
	case f.nodeModules:
		v := true
		o.NodeModules = &v
	}
	return o
}

func projectRoot() string {
	d, _ := os.Getwd()
	return remoteclient.FindProjectRoot(d)
}

func open(ctx context.Context, remote string) (*Client, error) {
	rc, err := remoteclient.Open(projectRoot(), remote)
	if err != nil {
		return nil, err
	}
	wctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	c, err := NewClient(wctx, rc)
	if err != nil {
		return nil, err
	}
	// fail fast on a bad login before collecting files
	if _, err := c.remote().Status(wctx, extkit.Auth{}); err != nil && !remoteclient.IsStatus(err, http.StatusForbidden) {
		return nil, err // a deploy-only token may lack remote:read; any other failure is a bad login
	}
	return c, nil
}

func printer(w io.Writer) func(string) { return func(s string) { fmt.Fprintln(w, s) } }

func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

func deployCommand() *cobra.Command {
	var (
		f       setFlags
		commit  string
		message string
		out     string
		dryRun  bool
		restart bool
	)
	c := &cobra.Command{
		Use:   "deploy [remote]",
		Short: "Deploys pb_hooks, pb_migrations, pb_pages, pb_public and packages to a remote",
		Long: "Deploys the project's code set (pb_* folders except pb_data, package.json, bun.lock(b), patches/, and the " +
			"production node_modules tree) to a remote. Only changed files are uploaded and the remote swaps them in. " +
			"Hooks, pages and public files take effect by themselves; migrations and packages apply when the app " +
			"restarts: pass --restart, or run '" + extkit.Command + " restart' later.",
		Args: cobra.MaximumNArgs(1), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := printer(cmd.OutOrStdout())
			opts := f.collect(projectRoot())
			if dryRun || out != "" {
				set, err := Collect(opts)
				if err != nil {
					return err
				}
				for _, l := range set.Describe() {
					log(l)
				}
				if err := set.CheckComplete(); err != nil {
					return err
				}
				if dryRun {
					var total int64
					for _, fl := range set.Files {
						if f.verbose || !strings.HasPrefix(fl.Path, "node_modules/") {
							log("   " + fl.Path)
						}
						total += fl.Size
					}
					log(fmt.Sprintf("Would deploy %d files (%s).", len(set.Files), humanBytes(total)))
					return nil
				}
				if err := set.WriteTo(out); err != nil {
					return err
				}
				log(fmt.Sprintf("Wrote %d files to %s.", len(set.Files), out))
				return nil
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			c, err := open(ctx, firstArg(args))
			if err != nil {
				return err
			}
			_, err = Deploy(ctx, c, DeployOptions{CollectOptions: opts, Commit: commit, Message: message, Verbose: f.verbose, Log: log, Restart: restart})
			return err
		},
	}
	f.bind(c)
	c.Flags().StringVar(&commit, "commit", "", "commit SHA to record (default: $GITHUB_SHA, $CI_COMMIT_SHA, or git HEAD)")
	c.Flags().StringVarP(&message, "message", "m", "", "note to record with this deploy")
	c.Flags().StringVar(&out, "out", "", "write the deploy set to a local folder instead of deploying")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "list what would be deployed, deploy nothing")
	c.Flags().BoolVar(&restart, "restart", false, "restart the app after deploying when migrations or packages changed")
	return c
}

func restartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restart [remote]",
		Short: "Restarts a remote's app so it applies new migrations and packages",
		Args:  cobra.MaximumNArgs(1), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			c, err := open(ctx, firstArg(args))
			if err != nil {
				return err
			}
			res, err := c.Restart(ctx)
			if err != nil {
				return err
			}
			printer(cmd.OutOrStdout())(res.Message())
			if res.StartError != "" {
				return fmt.Errorf("the app failed to start")
			}
			return nil
		},
	}
}

func devCommand() *cobra.Command {
	var (
		f        setFlags
		noLogs   bool
		pushOnly bool
		prefer   string
		poll     time.Duration
	)
	c := &cobra.Command{
		Use:   "dev [remote]",
		Short: "Keeps the project's code and a remote in step both ways (and streams the remote's logs)",
		Long: "Syncs pb_hooks, pb_migrations, pb_pages, pb_public and the other pb_* folders, package.json, bun.lock(b) and " +
			"patches/ both ways: edits here are uploaded half a second after you save, and changes on the remote (such " +
			"as migrations the dashboard writes) are downloaded within seconds. node_modules is only uploaded, when " +
			"package.json or the lockfile changes. pb_data is never synced. Hooks, pages and public files take effect " +
			"by themselves; migrations and packages you upload apply when the app restarts: type r and Enter. State " +
			"is kept in .pocketone/; files to leave out go in .pocketoneignore.",
		Args: cobra.MaximumNArgs(1), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			out := cmd.OutOrStdout()
			log := printer(out)
			c, err := open(ctx, firstArg(args))
			if err != nil {
				return err
			}
			opts := f.collect(projectRoot())
			if prefer != "" && prefer != "local" && prefer != "remote" {
				return fmt.Errorf("--prefer must be local or remote")
			}
			if !pushOnly {
				if _, err := c.Tree(ctx, "", true); errors.Is(err, ErrNoSyncAPI) {
					log(c.Remote.URL + " doesn't support two-way sync yet; uploading changes only.")
					pushOnly = true
				} else if err != nil {
					return err
				}
			}
			if !noLogs {
				go followLogs(ctx, c, 0, out)
			}
			hint := "type r and Enter to restart the app and apply them."
			if pushOnly {
				go restartKey(ctx, c, log)
				return Watch(ctx, opts, 500*time.Millisecond, log, func() {
					if _, err := Deploy(ctx, c, DeployOptions{CollectOptions: opts, Message: "dev", Verbose: f.verbose, Log: log, RestartHint: hint}); err != nil && ctx.Err() == nil {
						log("Deploy failed: " + err.Error())
					}
				})
			}
			s, err := NewSyncer(c, SyncOptions{CollectOptions: opts, Prefer: prefer, Ask: terminalAsk(cmd), Verbose: f.verbose, Log: log, RestartHint: hint})
			if err != nil {
				return err
			}
			log("Syncing code with " + c.Remote.URL + ": pb_* folders, package.json, lockfile, patches/ (node_modules: upload only)")
			log("Not synced: pb_data (the app's live data)")
			if err := s.Sync(ctx); err != nil {
				return err
			}
			go restartKey(ctx, c, log)
			go func() {
				t := time.NewTicker(poll)
				defer t.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-t.C:
						changed, err := s.RemoteChanged(ctx)
						if err == nil && changed {
							err = s.Sync(ctx)
						}
						if err != nil && ctx.Err() == nil {
							log("Sync failed: " + err.Error())
						}
					}
				}
			}()
			first := true
			return Watch(ctx, opts, 500*time.Millisecond, log, func() {
				if first { // Watch runs this once at start; the first sync already happened
					first = false
					return
				}
				if err := s.Sync(ctx); err != nil && ctx.Err() == nil {
					log("Sync failed: " + err.Error())
				}
			})
		},
	}
	f.bind(c)
	c.Flags().BoolVar(&noLogs, "no-logs", false, "don't stream the remote's logs")
	c.Flags().BoolVar(&pushOnly, "push-only", false, "only upload changes (the old one-way dev); never download")
	c.Flags().StringVar(&prefer, "prefer", "", "on the first sync, files that differ: local (upload yours) or remote (download theirs); default asks")
	c.Flags().DurationVar(&poll, "poll", DefaultSyncPoll, "how often to check the remote for changes")
	return c
}

// restartKey restarts the remote app when the user types r and Enter (dev). It reads stdin only when it's a terminal.
func restartKey(ctx context.Context, c *Client, log func(string)) {
	st, err := os.Stdin.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return
	}
	log("Type r and Enter to restart the app (applies migrations and packages).")
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() && ctx.Err() == nil {
		if strings.TrimSpace(sc.Text()) != "r" {
			continue
		}
		log("Restarting the app...")
		res, err := c.Restart(ctx)
		if err != nil {
			if ctx.Err() == nil {
				log("Restart failed: " + err.Error())
			}
			continue
		}
		log(res.Message())
	}
}

// terminalAsk asks on stdin when it's a terminal; nil otherwise (then --prefer is needed).
func terminalAsk(cmd *cobra.Command) func(string) string {
	st, err := os.Stdin.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return nil
	}
	return func(q string) string {
		fmt.Fprint(cmd.OutOrStdout(), q)
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		return line
	}
}

// Watch calls deploy once, then again (debounced) whenever a file of the deploy set changes, until ctx ends.
func Watch(ctx context.Context, opts CollectOptions, debounce time.Duration, log func(string), deploy func()) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	include := opts.Include
	if len(include) == 0 {
		include = DefaultIncludes
	}
	root, _ := filepath.Abs(opts.Dir)
	watchable := func(rel string) bool {
		if rel == "." {
			return true
		}
		name := filepath.Base(rel)
		if name == "pb_data" || name == "node_modules" || strings.HasPrefix(name, ".git") {
			return false // a package install changes package.json or the lockfile too
		}
		return glob.AnyUnder(include, rel)
	}
	addTree := func(dir string) {
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			if !watchable(rel) {
				return filepath.SkipDir
			}
			_ = w.Add(p)
			return nil
		})
	}
	addTree(root)

	var (
		mu    sync.Mutex
		timer *time.Timer
		busy  sync.Mutex
	)
	trigger := func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(debounce, func() {
			busy.Lock()
			defer busy.Unlock()
			deploy()
		})
	}
	busy.Lock()
	deploy()
	busy.Unlock()
	log("Watching for changes (Ctrl+C to stop)…")
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-w.Errors:
			if err != nil {
				log("watch: " + err.Error())
			}
		case ev, ok := <-w.Events:
			if !ok {
				return errors.New("watcher closed")
			}
			rel, _ := filepath.Rel(root, ev.Name)
			rel = filepath.ToSlash(rel)
			if ev.Op&fsnotify.Create != 0 {
				if st, err := os.Stat(ev.Name); err == nil && st.IsDir() {
					addTree(ev.Name)
				}
			}
			if strings.HasPrefix(rel, "pb_data/") || strings.Contains("/"+rel, "/.git") {
				continue
			}
			trigger()
		}
	}
}

func pullCommand() *cobra.Command {
	var dryRun bool
	c := &cobra.Command{
		Use:          "pull [remote] [paths...]",
		Short:        "Downloads files from a remote into the project (default: pb_hooks, pb_migrations, pb_pages, pb_public, pb_templates, package.json)",
		Example:      "pull prod\npull prod pb_hooks pb_public/index.html",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			c, err := open(ctx, firstArg(args))
			if err != nil {
				return err
			}
			var paths []string
			if len(args) > 1 {
				paths = args[1:]
			}
			log := printer(cmd.OutOrStdout())
			n, err := Pull(ctx, c, PullOptions{Dir: projectRoot(), Paths: paths, DryRun: dryRun, Log: log})
			if err != nil {
				return err
			}
			if dryRun {
				log(fmt.Sprintf("Would pull %d file(s).", n))
			} else {
				log(fmt.Sprintf("Pulled %d file(s) from %s.", n, c.Remote.URL))
			}
			return nil
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "list what would be pulled")
	return c
}

func logsCommand() *cobra.Command {
	var n int
	c := &cobra.Command{
		Use:   "logs [remote]",
		Short: "Streams a remote's app logs",
		Args:  cobra.MaximumNArgs(1), SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			c, err := open(ctx, firstArg(args))
			if err != nil {
				return err
			}
			followLogs(ctx, c, n, cmd.OutOrStdout())
			return nil
		},
	}
	c.Flags().IntVarP(&n, "lines", "n", 100, "replay this many earlier lines (0..1000)")
	return c
}

// followLogs streams logs and reconnects (without replay) when the stream drops, until ctx ends.
func followLogs(ctx context.Context, c *Client, n int, out io.Writer) {
	for ctx.Err() == nil {
		err := c.StreamLogs(ctx, n, func(e LogEntry) {
			prefix := ""
			if e.Stream == "stderr" {
				prefix = "ERR "
			}
			ts := e.Time
			if t, err := time.Parse(time.RFC3339Nano, e.Time); err == nil {
				ts = t.Local().Format("15:04:05")
			}
			fmt.Fprintf(out, "%s %s%s\n", ts, prefix, strings.TrimRight(e.Message, "\n"))
		})
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			fmt.Fprintf(out, "log stream: %v (reconnecting)\n", err)
		}
		n = 0
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
}
