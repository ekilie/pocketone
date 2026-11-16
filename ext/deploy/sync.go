package deploy

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	remoteclient "github.com/pockethost/pocketone/ext/contracts/remote/v1/remoteclient"
	"github.com/pockethost/pocketone/ext/deploy/internal/glob"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Two-way sync for `dev`: the project's code set (pb_* folders except pb_data, package.json, bun.lock(b), patches/)
// is kept the same locally and on the remote. node_modules is only pushed (built from package.json and the lockfile),
// pb_data is never synced.
//
// .pocketone/sync.json keeps each file's hash as of the last sync, so every file falls in one case: changed on one
// side (copy it over), deleted on one side (delete it on the other), or changed on both (keep yours, save the
// server's copy under .pocketone/conflicts/ and say so). The remote is watched by polling the hash of its /tree root
// and walking down only into folders whose hash changed.

const (
	syncStateDir  = ".pocketone"
	syncStateFile = "sync.json"
	conflictsDir  = "conflicts"
	ignoreFile    = ".pocketoneignore"
	// DefaultSyncPoll is how often `dev` checks the remote for changes.
	DefaultSyncPoll = 5 * time.Second
)

// ErrNoSyncAPI: the remote's deploy API has no /tree (an older PocketOne or the legacy PocketHost edge).
var ErrNoSyncAPI = errors.New("the remote doesn't support two-way sync")

// Tree fetches one folder of the remote's code set (hashOnly: just its hash).
func (c *Client) Tree(ctx context.Context, p string, hashOnly bool) (*TreeNode, error) {
	q := url.Values{"path": {p}}
	if hashOnly {
		q.Set("hashOnly", "1")
	}
	var n TreeNode
	err := c.call(ctx, http.MethodGet, "/tree?"+q.Encode(), nil, &n)
	if isNotFound(err) {
		return nil, ErrNoSyncAPI
	}
	return &n, err
}

type syncState struct {
	Remote string            `json:"remote"`
	Files  map[string]string `json:"files"`
}

// SyncOptions configure a [Syncer].
type SyncOptions struct {
	CollectOptions
	// Prefer settles files that differ on the first sync: "local", "remote", or "" to ask (Ask) or fail.
	Prefer string
	// Ask shows a question and returns the answer ("l", "r" or "q"); nil when there's no terminal.
	Ask     func(question string) string
	Verbose bool
	Log     func(string)
	// RestartHint is printed after a push that wrote a migration or changed packages (default: how to restart).
	RestartHint string
}

// Syncer keeps a project folder and a remote in step. Call Sync after a local change or on a timer; it's safe from
// several goroutines (calls run one at a time).
type Syncer struct {
	c      *Client
	opts   SyncOptions
	dir    string
	ignore []string
	hashes HashCache

	mu        sync.Mutex
	state     syncState
	firstRun  bool
	nodes     map[string]*TreeNode // remote folders by path, as last fetched
	remote    map[string]string    // remote files: path -> hash
	remoteTop string
}

// NewSyncer loads the sync state and the ignore file.
func NewSyncer(c *Client, opts SyncOptions) (*Syncer, error) {
	if opts.Log == nil {
		opts.Log = func(string) {}
	}
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	s := &Syncer{c: c, opts: opts, dir: dir, nodes: map[string]*TreeNode{}}
	s.ignore, err = readIgnore(filepath.Join(dir, ignoreFile))
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.statePath())
	if err == nil {
		_ = json.Unmarshal(b, &s.state)
	}
	if s.state.Files == nil || s.state.Remote != c.Remote.URL {
		s.state = syncState{Remote: c.Remote.URL, Files: map[string]string{}}
		s.firstRun = true
	}
	return s, nil
}

// readIgnore reads gitignore-style lines (blank lines and # comments skipped). A pattern also matches everything
// below a folder of that name; a pattern without a slash matches at any depth.
func readIgnore(p string) ([]string, error) {
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		l = strings.TrimSuffix(strings.TrimPrefix(l, "/"), "/")
		if !strings.Contains(l, "/") {
			l = "**/" + l
		}
		out = append(out, l, l+"/**")
	}
	return out, sc.Err()
}

func (s *Syncer) statePath() string { return filepath.Join(s.dir, syncStateDir, syncStateFile) }

func (s *Syncer) saveState() error {
	if err := os.MkdirAll(filepath.Dir(s.statePath()), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.statePath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.statePath())
}

// synced reports whether a path takes part in the sync (the deploy set's patterns, minus the ignore file).
func (s *Syncer) synced(p string) bool {
	include := s.opts.Include
	if len(include) == 0 {
		include = DefaultIncludes
	}
	exclude := append(append(append([]string{}, DefaultExcludes...), s.opts.Exclude...), s.ignore...)
	return isCodePath(p) && !isNodeModulesPath(p) && glob.Any(include, p) && !glob.Any(exclude, p)
}

// local lists the project's synced files (never node_modules).
func (s *Syncer) local() (map[string]LocalFile, error) {
	no := false
	o := s.opts.CollectOptions
	o.Dir, o.NodeModules, o.Hashes = s.dir, &no, &s.hashes
	o.Exclude = append(append([]string{}, o.Exclude...), s.ignore...)
	set, err := Collect(o)
	if err != nil {
		return nil, err
	}
	out := make(map[string]LocalFile, len(set.Files))
	for _, f := range set.Files {
		out[f.Path] = f
	}
	return out, nil
}

// RemoteChanged asks the remote for its root hash only: true when it differs from the last one seen.
func (s *Syncer) RemoteChanged(ctx context.Context) (bool, error) {
	n, err := s.c.Tree(ctx, "", true)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.remote == nil || n.Hash != s.remoteTop, nil
}

// fetchRemote refreshes the remote file list, fetching only folders whose hash changed since the last call.
func (s *Syncer) fetchRemote(ctx context.Context) error {
	root, err := s.c.Tree(ctx, "", false)
	if err != nil {
		return err
	}
	if s.remote != nil && root.Hash == s.remoteTop {
		return nil
	}
	nodes := map[string]*TreeNode{"": root}
	var walk func(n *TreeNode) error
	walk = func(n *TreeNode) error {
		for _, e := range n.Entries {
			if !e.Dir {
				continue
			}
			p := path.Join(n.Path, e.Name)
			child := s.nodes[p]
			if child == nil || child.Hash != e.Hash {
				if child, err = s.c.Tree(ctx, p, false); err != nil {
					return err
				}
			}
			nodes[p] = child
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return err
	}
	files := map[string]string{}
	for p, n := range nodes {
		for _, e := range n.Entries {
			if fp := path.Join(p, e.Name); !e.Dir && s.synced(fp) {
				files[fp] = e.Hash
			}
		}
	}
	s.nodes, s.remote, s.remoteTop = nodes, files, root.Hash
	return nil
}

// syncPlan is what one Sync does.
type syncPlan struct {
	push, pushDelete, pull, pullDelete []string
	conflicts                          []string // changed on both sides: keep local, save the remote copy
	firstDiffs                         []string // differ on the first sync, with no history to tell who changed
	forget                             []string // gone on both sides
}

func (p *syncPlan) empty() bool {
	return len(p.push)+len(p.pushDelete)+len(p.pull)+len(p.pullDelete)+len(p.conflicts)+len(p.firstDiffs)+len(p.forget) == 0
}

// planSync compares local, remote and the last synced hashes.
func planSync(local, remote, base map[string]string) syncPlan {
	var p syncPlan
	paths := map[string]bool{}
	for _, m := range []map[string]string{local, remote, base} {
		for k := range m {
			paths[k] = true
		}
	}
	sorted := make([]string, 0, len(paths))
	for k := range paths {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, f := range sorted {
		l, lok := local[f]
		r, rok := remote[f]
		b, bok := base[f]
		switch {
		case lok && rok && l == r:
			// agreed (base updated by the caller)
		case !lok && !rok:
			p.forget = append(p.forget, f)
		case !bok:
			switch {
			case lok && !rok:
				p.push = append(p.push, f)
			case !lok && rok:
				p.pull = append(p.pull, f)
			default:
				p.firstDiffs = append(p.firstDiffs, f)
			}
		default:
			lChanged := !lok || l != b
			rChanged := !rok || r != b
			switch {
			case lChanged && !rChanged:
				if lok {
					p.push = append(p.push, f)
				} else {
					p.pushDelete = append(p.pushDelete, f)
				}
			case rChanged && !lChanged:
				if rok {
					p.pull = append(p.pull, f)
				} else {
					p.pullDelete = append(p.pullDelete, f)
				}
			case !lok: // deleted here, changed there: the change wins
				p.pull = append(p.pull, f)
			case !rok: // changed here, deleted there
				p.push = append(p.push, f)
			default:
				p.conflicts = append(p.conflicts, f)
			}
		}
	}
	return p
}

// Sync brings both sides in step once.
func (s *Syncer) Sync(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	log := s.opts.Log

	if err := s.fetchRemote(ctx); err != nil {
		return err
	}
	localFiles, err := s.local()
	if err != nil {
		return err
	}
	local := make(map[string]string, len(localFiles))
	for p, f := range localFiles {
		local[p] = f.Hash
	}
	plan := planSync(local, s.remote, s.state.Files)

	if len(plan.firstDiffs) > 0 {
		keepLocal, err := s.settleFirstDiffs(plan.firstDiffs)
		if err != nil {
			return err
		}
		if keepLocal {
			plan.push = append(plan.push, plan.firstDiffs...)
		} else {
			plan.pull = append(plan.pull, plan.firstDiffs...)
		}
		plan.firstDiffs = nil
	}
	shipNodeModules := s.firstRun && s.hasPackageJSON()
	for _, p := range append(append([]string{}, plan.push...), plan.conflicts...) {
		if p == "package.json" || p == "bun.lock" || p == "bun.lockb" {
			shipNodeModules = true
		}
	}

	changed := !plan.empty()
	// agreed files: record them
	for p, h := range local {
		if r, ok := s.remote[p]; ok && r == h {
			s.state.Files[p] = h
		}
	}
	for _, p := range plan.forget {
		delete(s.state.Files, p)
	}

	// conflicts: save the server's copy aside, then push ours
	for _, p := range plan.conflicts {
		dest := filepath.Join(s.dir, syncStateDir, conflictsDir, filepath.FromSlash(p))
		if err := s.c.Download(ctx, p, dest); err != nil {
			return fmt.Errorf("saving the server's copy of %s: %w", p, err)
		}
		rel, _ := filepath.Rel(s.dir, dest)
		log(fmt.Sprintf("! %s changed here and on the server. Kept yours; the server's copy is in %s.", p, filepath.ToSlash(rel)))
		plan.push = append(plan.push, p)
	}

	if err := s.pull(ctx, plan.pull, localFiles); err != nil {
		return err
	}
	for _, p := range plan.pullDelete {
		if f, ok := localFiles[p]; ok {
			if h, _, err := s.hashes.hash(f.Source); err == nil && h != f.Hash {
				continue // edited since the scan: the next sync sees it
			}
			if err := os.Remove(f.Source); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			pruneEmptyParents(s.dir, f.Source)
		}
		delete(s.state.Files, p)
		log("↓ deleted " + p)
	}
	if len(plan.push) > 0 || len(plan.pushDelete) > 0 || shipNodeModules {
		if err := s.push(ctx, plan.push, plan.pushDelete, localFiles, shipNodeModules); err != nil {
			return err
		}
		// take in our own change, so the next poll doesn't see it as the server's
		if err := s.fetchRemote(ctx); err != nil {
			return err
		}
	}
	s.firstRun = false
	if changed || shipNodeModules {
		return s.saveState()
	}
	if _, err := os.Stat(s.statePath()); errors.Is(err, os.ErrNotExist) {
		return s.saveState()
	}
	return nil
}

func (s *Syncer) hasPackageJSON() bool {
	_, err := os.Stat(filepath.Join(s.dir, "package.json"))
	return err == nil
}

// settleFirstDiffs decides who wins for files that differ on the first sync (no history to tell who changed them).
func (s *Syncer) settleFirstDiffs(diffs []string) (keepLocal bool, err error) {
	log := s.opts.Log
	log(fmt.Sprintf("%d file(s) differ between this folder and %s:", len(diffs), s.c.Remote.URL))
	for i, p := range diffs {
		if i == 20 {
			log(fmt.Sprintf("   … and %d more", len(diffs)-20))
			break
		}
		log("   " + p)
	}
	switch s.opts.Prefer {
	case "local":
		return true, nil
	case "remote":
		return false, nil
	}
	if s.opts.Ask == nil {
		return false, errors.New("run again with --prefer local (upload yours) or --prefer remote (download the server's)")
	}
	switch strings.ToLower(strings.TrimSpace(s.opts.Ask("Keep [l]ocal files (upload them), take the [r]emote ones (download), or [q]uit? "))) {
	case "l", "local":
		return true, nil
	case "r", "remote":
		return false, nil
	}
	return false, errors.New("stopped; nothing was changed")
}

func (s *Syncer) pull(ctx context.Context, paths []string, localFiles map[string]LocalFile) error {
	for _, p := range paths {
		dest := filepath.Join(s.dir, filepath.FromSlash(p))
		if f, ok := localFiles[p]; ok {
			if h, _, err := s.hashes.hash(f.Source); err == nil && h != f.Hash {
				continue // edited since the scan: the next sync sees it
			}
		}
		if err := s.c.Download(ctx, p, dest); err != nil {
			var ae *remoteclient.APIError
			if errors.As(err, &ae) && ae.Status == http.StatusNotFound {
				continue // gone since the listing
			}
			return fmt.Errorf("pulling %s: %w", p, err)
		}
		h, _, err := s.hashes.hash(dest)
		if err != nil {
			return err
		}
		s.state.Files[p] = h
		s.opts.Log("↓ " + p)
	}
	return nil
}

func (s *Syncer) push(ctx context.Context, paths, deletes []string, localFiles map[string]LocalFile, nodeModules bool) error {
	log := s.opts.Log
	manifest := map[string]string{}
	byHash := map[string]LocalFile{}
	for _, p := range paths {
		f := localFiles[p]
		manifest[p] = f.Hash
		byHash[f.Hash] = f
	}
	if nodeModules {
		yes := true
		set, err := Collect(CollectOptions{Dir: s.dir, Include: []string{"package.json"}, NodeModules: &yes, Hashes: &s.hashes})
		if err != nil {
			return err
		}
		if err := set.CheckComplete(); err != nil {
			return err
		}
		for _, f := range set.Files {
			if isNodeModulesPath(f.Path) {
				manifest[f.Path] = f.Hash
				byHash[f.Hash] = f
			}
		}
	}
	up := &uploader{c: s.c, byHash: byHash, verbose: s.opts.Verbose, log: log}
	missing, err := s.c.Check(ctx, manifest, nodeModules)
	if err != nil {
		return err
	}
	if err := up.send(ctx, missing); err != nil {
		return err
	}
	if deletes == nil {
		deletes = []string{}
	}
	res, err := up.commit(ctx, map[string]any{
		"files": manifest, "partial": true, "delete": deletes, "nodeModules": nodeModules,
		"message": "dev", "source": "pocketone-dev", "commit": detectCommit(s.dir),
	})
	if err != nil {
		return err
	}
	for _, p := range paths {
		s.state.Files[p] = manifest[p]
		log("↑ " + p)
	}
	for _, p := range deletes {
		delete(s.state.Files, p)
		log("↑ deleted " + p)
	}
	switch {
	case res.Restarted:
		log("Restarted the app.")
	case res.StartError != "":
		log("The app failed to start: " + res.StartError)
	case res.RestartNeeded:
		log("Migrations or packages changed: " + orStr(s.opts.RestartHint,
			"restart the app ('"+extkit.Command+" restart') to apply them."))
	}
	return nil
}

// pruneEmptyParents removes now-empty folders above a deleted file, never a top-level one (pb_hooks etc.).
func pruneEmptyParents(root, file string) {
	for d := filepath.Dir(file); ; d = filepath.Dir(d) {
		rel, err := filepath.Rel(root, d)
		if err != nil || rel == "." || !strings.Contains(filepath.ToSlash(rel), "/") {
			return
		}
		if os.Remove(d) != nil {
			return
		}
	}
}
