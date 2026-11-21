package deploy

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	deployv1 "pocketone.dev/ext/contracts/deploy/v1"
	fsv1 "pocketone.dev/ext/contracts/instance/fs/v1"
	"pocketone.dev/ext/extkit"
	"slices"
	"strings"
	"sync"

	remoteclient "pocketone.dev/ext/contracts/remote/v1/remoteclient"
	"pocketone.dev/ext/deploy/internal/glob"
	"pocketone.dev/ext/deploy/nodemodules"
)

// UploadBatchBytes is the raw size of one upload request (gzip shrinks it; Cloudflare allows 100 MB).
var UploadBatchBytes int64 = 32 << 20

// DefaultIncludes and DefaultExcludes are the npm CLI's deploy set.
var (
	DefaultIncludes = []string{"pb_*", "pb_*/**/*", "package.json", "bun.lockb", "bun.lock", "patches", "patches/**/*"}
	DefaultExcludes = []string{"pb_data", "pb_data/**/*", "pb_cache", "pb_cache/**/*"}
)

// LocalFile is one file of a deploy set.
type LocalFile struct {
	Path   string // logical path from the project root
	Hash   string // sha256 hex
	Size   int64
	Source string // real file on disk (differs from Path for node_modules entries)
}

// DeploySet is what a deploy would send.
type DeploySet struct {
	Files []LocalFile
	// Skipped: files the include patterns matched that aren't deployable (outside the code set, symlinks).
	Skipped []string
	// NodeModules is the collected production tree, nil when the deploy doesn't manage node_modules.
	NodeModules *nodemodules.Result
}

// CollectOptions pick the files.
type CollectOptions struct {
	Dir     string   // project root (default: the current folder)
	Include []string // default DefaultIncludes
	Exclude []string // added to DefaultExcludes
	// NodeModules: ship the production node_modules tree and let the remote manage its root node_modules. Nil:
	// when the project has a package.json.
	NodeModules *bool
	// Hashes, if set, caches file hashes across calls (by path, size and mtime), so repeated collects only re-hash
	// files that changed.
	Hashes *HashCache
}

// HashCache remembers local file hashes by path, size and mtime. The zero value is ready to use; a nil cache hashes
// every time.
type HashCache struct {
	mu sync.Mutex
	m  map[string]hashEntry
}

func (c *HashCache) hash(p string) (string, int64, error) {
	if c == nil {
		return hashFile(p)
	}
	st, err := os.Stat(p)
	if err != nil {
		return "", 0, err
	}
	c.mu.Lock()
	e, ok := c.m[p]
	c.mu.Unlock()
	if ok && e.size == st.Size() && e.mtime == st.ModTime().UnixNano() {
		return e.hash, e.size, nil
	}
	h, n, err := hashFile(p)
	if err != nil {
		return "", 0, err
	}
	c.mu.Lock()
	if c.m == nil {
		c.m = map[string]hashEntry{}
	}
	c.m[p] = hashEntry{st.Size(), st.ModTime().UnixNano(), h}
	c.mu.Unlock()
	return h, n, nil
}

// Collect lists and hashes the deploy set: files matching the include patterns and not the exclude patterns,
// within the code set (pb_* except pb_data and pb_cache, package.json, bun.lock(b), patches/), plus the production node_modules
// tree. Dotfiles ship (pb_public/.well-known); .git* files and symlinks don't.
func Collect(opts CollectOptions) (*DeploySet, error) {
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	include := opts.Include
	if len(include) == 0 {
		include = DefaultIncludes
	}
	exclude := append(slices.Clone(DefaultExcludes), opts.Exclude...)

	set := &DeploySet{}
	err = filepath.WalkDir(dir, func(abs string, d fs.DirEntry, err error) error {
		if err != nil {
			if abs == dir {
				return err
			}
			return nil
		}
		rel, _ := filepath.Rel(dir, abs)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || strings.HasPrefix(name, ".git") || glob.Any(exclude, rel) || !glob.AnyUnder(include, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !glob.Any(include, rel) || glob.Any(exclude, rel) {
			return nil
		}
		if !d.Type().IsRegular() || !fsv1.IsCodePath(rel) {
			set.Skipped = append(set.Skipped, rel)
			return nil
		}
		h, size, err := opts.Hashes.hash(abs)
		if err != nil {
			return err
		}
		set.Files = append(set.Files, LocalFile{Path: rel, Hash: h, Size: size, Source: abs})
		return nil
	})
	if err != nil {
		return nil, err
	}

	manage := opts.NodeModules
	if manage == nil {
		_, err := os.Stat(filepath.Join(dir, "package.json"))
		has := err == nil
		manage = &has
	}
	if *manage {
		nm, err := nodemodules.Collect(dir)
		if err != nil {
			return nil, err
		}
		set.NodeModules = nm
		for _, f := range nm.Files {
			h, size, err := opts.Hashes.hash(f.Source)
			if err != nil {
				return nil, err
			}
			set.Files = append(set.Files, LocalFile{Path: f.Path, Hash: h, Size: size, Source: f.Source})
		}
	}
	return set, nil
}

func hashFile(p string) (string, int64, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// Manifest maps each path to its hash, as the API expects.
func (s *DeploySet) Manifest() map[string]string {
	m := make(map[string]string, len(s.Files))
	for _, f := range s.Files {
		m[f.Path] = f.Hash
	}
	return m
}

// Describe returns human lines about the node_modules part, plus a warning for missing packages.
func (s *DeploySet) Describe() []string {
	nm := s.NodeModules
	if nm == nil {
		return nil
	}
	var lines []string
	if len(nm.Packages) == 0 {
		lines = append(lines, "node_modules: no packages (any on the remote will be removed)")
	} else {
		lines = append(lines, fmt.Sprintf("node_modules: %d package(s), %d file(s)", len(nm.Packages), len(nm.Files)))
	}
	if len(nm.Missing) > 0 {
		var parts []string
		for _, m := range nm.Missing {
			parts = append(parts, m.Name+" (required by "+m.RequiredBy+")")
		}
		lines = append(lines, fmt.Sprintf("%d required package(s) not installed: %s. Run your package manager's install first.",
			len(nm.Missing), strings.Join(parts, ", ")))
	}
	return lines
}

// CheckComplete fails when required packages are missing.
func (s *DeploySet) CheckComplete() error {
	if s.NodeModules != nil && len(s.NodeModules.Missing) > 0 {
		return fmt.Errorf("not deploying: %d required package(s) are missing from node_modules", len(s.NodeModules.Missing))
	}
	return nil
}

// WriteTo copies the deploy set into outDir (empty or new) as plain files: logical paths, symlinks resolved.
func (s *DeploySet) WriteTo(outDir string) error {
	if entries, err := os.ReadDir(outDir); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s must be empty or not exist yet", outDir)
	}
	for _, f := range s.Files {
		dest := filepath.Join(outDir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := copyFile(f.Source, dest); err != nil {
			return err
		}
	}
	return nil
}

// EncodeBundle builds an upload body: "PHB1" + [32-byte sha256][uint32 BE length][bytes] per entry (gzip it).
func EncodeBundle(entries [][]byte) []byte {
	var b bytes.Buffer
	b.Write(bundleMagic)
	for _, data := range entries {
		sum := sha256.Sum256(data)
		b.Write(sum[:])
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(data)))
		b.Write(n[:])
		b.Write(data)
	}
	return b.Bytes()
}

// BatchBySize groups files into batches of at most max raw bytes (a larger single file gets its own batch).
func BatchBySize(files []LocalFile, max int64) [][]LocalFile {
	var out [][]LocalFile
	var cur []LocalFile
	var curBytes int64
	for _, f := range files {
		if len(cur) > 0 && curBytes+f.Size > max {
			out = append(out, cur)
			cur, curBytes = nil, 0
		}
		cur = append(cur, f)
		curBytes += f.Size
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// ---------------------------------------------------------------------------------------------------------------
// API client

// Client is the deploy API of one remote.
type Client struct {
	*remoteclient.Client
	// Prefix is the deploy API's path: deploy/v1's (/api/_c/deploy/v1).
	Prefix string
}

// NewClient finds the remote's deploy API: the remote must offer remote/v1 (restart, logs) and deploy/v1 (files).
func NewClient(ctx context.Context, rc *remoteclient.Client) (*Client, error) {
	w, err := rc.Whoami(ctx)
	if errors.Is(err, remoteclient.ErrNoWhoami) {
		return nil, fmt.Errorf("%s doesn't offer remote/v1 (it isn't a PocketOne app, or runs an older one)", rc.Remote.URL)
	}
	if err != nil {
		return nil, err
	}
	if !w.Offers("deploy/v1") {
		return nil, fmt.Errorf("%s doesn't offer deploy/v1 (no deploy extension)", rc.Remote.URL)
	}
	return &Client{Client: rc, Prefix: deployv1.Prefix}, nil
}

func (c *Client) call(ctx context.Context, method, path string, in, out any) error {
	err := c.JSON(ctx, method, c.Prefix+path, in, out)
	var ae *remoteclient.APIError
	if errors.As(err, &ae) {
		switch ae.Status {
		case http.StatusUnauthorized:
			ae.Message = fmt.Sprintf("authentication failed (%s): %s", orStr(c.TokenSource, "no login"), ae.Message)
		case http.StatusNotFound:
			if path == "/check" {
				ae.Message = fmt.Sprintf("%s: not found, or this login can't reach it (%s)", c.Remote.URL, ae.Message)
			}
		}
	}
	return err
}

func orStr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// Check returns the hashes the remote doesn't have.
func (c *Client) Check(ctx context.Context, manifest map[string]string, nodeModules bool) ([]string, error) {
	var res struct {
		Missing []string `json:"missing"`
	}
	err := c.call(ctx, http.MethodPost, "/check", map[string]any{"files": manifest, "nodeModules": nodeModules}, &res)
	return res.Missing, err
}

// Upload sends one gzipped bundle.
func (c *Client) Upload(ctx context.Context, gz []byte) error {
	return c.call(ctx, http.MethodPost, "/blobs", gz, nil)
}

// CommitResult is the remote's answer to a commit.
type CommitResult struct {
	DeployId            *string  `json:"deployId"`
	FileCount           int      `json:"fileCount"`
	Written             []string `json:"written"`
	Deleted             []string `json:"deleted"`
	WrittenCount        int      `json:"writtenCount"`
	DeletedCount        int      `json:"deletedCount"`
	Unchanged           int      `json:"unchanged"`
	TotalBytes          int64    `json:"totalBytes"`
	ReplacedNodeModules bool     `json:"replacedNodeModules"`
	NothingChanged      bool     `json:"nothingChanged"`
	WasRunning          bool     `json:"wasRunning"`
	Restarted           bool     `json:"restarted"`
	StartError          string   `json:"startError,omitempty"`
	RestartNeeded       bool     `json:"restartNeeded,omitempty"` // migrations or packages wait for a restart
}

// RestartResult is the remote's answer to [Client.Restart].
type RestartResult struct {
	WasRunning bool   `json:"wasRunning"`
	Restarted  bool   `json:"restarted"`
	StartError string `json:"startError,omitempty"`
}

// ErrNoRestartAPI: the remote can't restart on request (an older PocketOne or PocketHost edge).
var ErrNoRestartAPI = errors.New("this remote can't restart on request")

// Restart restarts the remote app so it applies migrations and packages, through remote/v1.
func (c *Client) Restart(ctx context.Context) (*RestartResult, error) {
	r, err := c.remote().Restart(ctx, extkit.Auth{})
	if remoteclient.IsStatus(err, http.StatusNotFound) {
		return nil, ErrNoRestartAPI
	}
	if err != nil {
		return nil, err
	}
	return &RestartResult{Restarted: r.Restarting}, nil
}

// Message describes the restart for the user.
func (r *RestartResult) Message() string {
	switch {
	case r.Restarted:
		return "Restarting the app (follow it with '" + extkit.Command + " logs')."
	case r.StartError != "":
		return "The app failed to start: " + r.StartError + " (see '" + extkit.Command + " logs')."
	default:
		return "The app isn't running; it applies the changes when it starts."
	}
}

// isNotFound: the remote doesn't have the route (an older server).
func isNotFound(err error) bool {
	var ae *remoteclient.APIError
	return errors.As(err, &ae) && (ae.Status == http.StatusNotFound || ae.Status == http.StatusMethodNotAllowed)
}

// Commit asks the remote to swap the files in.
func (c *Client) Commit(ctx context.Context, body map[string]any) (*CommitResult, error) {
	var res CommitResult
	return &res, c.call(ctx, http.MethodPost, "/commit", body, &res)
}

// DeployOptions configure [Deploy].
type DeployOptions struct {
	CollectOptions
	Commit  string // recorded commit (default: $GITHUB_SHA, $CI_COMMIT_SHA, or git HEAD)
	Message string
	Source  string // recorded source (default: github-actions, ci, or pocketone-cli)
	Verbose bool
	Log     func(string)
	// Restart asks the remote to restart the app after the files are in, when migrations or packages changed.
	Restart bool
	// RestartHint is printed when migrations or packages changed and the app wasn't restarted (default: how to
	// restart it).
	RestartHint string
}

// Deploy sends the manifest, uploads only what the remote lacks, then commits. The remote swaps the files in; the app
// restarts only with opts.Restart, and only when migrations or packages changed (hooks reload by themselves).
func Deploy(ctx context.Context, c *Client, opts DeployOptions) (*CommitResult, error) {
	log := opts.Log
	if log == nil {
		log = func(string) {}
	}
	set, err := Collect(opts.CollectOptions)
	if err != nil {
		return nil, err
	}
	for _, l := range set.Describe() {
		log(l)
	}
	if err := set.CheckComplete(); err != nil {
		return nil, err
	}
	if len(set.Skipped) > 0 {
		msg := fmt.Sprintf("Skipping %d file(s) outside the deployable set (pb_* except pb_data and pb_cache, package.json, bun.lock(b), patches/)", len(set.Skipped))
		if opts.Verbose {
			msg += ": " + strings.Join(set.Skipped, ", ")
		}
		log(msg)
	}
	manifest := set.Manifest()
	byHash := map[string]LocalFile{}
	for _, f := range set.Files {
		byHash[f.Hash] = f
	}
	log(fmt.Sprintf("Deploying %d files to %s (%s)", len(set.Files), c.Remote.URL, orStr(c.TokenSource, "no login")))

	up := &uploader{c: c, byHash: byHash, verbose: opts.Verbose, log: log}
	managesNM := set.NodeModules != nil
	missing, err := c.Check(ctx, manifest, managesNM)
	if err != nil {
		return nil, err
	}
	if err := up.send(ctx, missing); err != nil {
		return nil, err
	}
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}
	body := map[string]any{
		"files": manifest, "nodeModules": managesNM,
		"commit": orStr(opts.Commit, detectCommit(dir)), "message": opts.Message,
		"source": orStr(opts.Source, detectSource()), "restart": opts.Restart,
	}
	res, err := up.commit(ctx, body)
	if err != nil {
		return nil, err
	}
	if opts.Verbose {
		for _, p := range res.Written {
			log("   + " + p)
		}
		for _, p := range res.Deleted {
			log("   - " + p)
		}
	}
	if res.NothingChanged {
		log("Up to date: no changes.")
		return res, nil
	}
	id := ""
	if res.DeployId != nil {
		id = " " + *res.DeployId
	}
	restart := ""
	switch {
	case res.Restarted:
		restart = " Restarted."
	case res.StartError != "":
		restart = " The app failed to start: " + res.StartError + " (see '" + extkit.Command + " logs')."
	case res.RestartNeeded:
		restart = " Migrations or packages changed: " + orStr(opts.RestartHint,
			"run '"+extkit.Command+" restart' (or deploy with --restart) to apply them.")
	}
	log(fmt.Sprintf("Deployed%s: %d changed, %d deleted, %d unchanged.%s", id, res.WrittenCount, res.DeletedCount, res.Unchanged, restart))
	return res, nil
}

// uploader sends the files a remote lacks, in gzipped bundles, and commits (uploading again once if blobs were
// garbage-collected in between).
type uploader struct {
	c       *Client
	byHash  map[string]LocalFile
	verbose bool
	log     func(string)
	count   int
	bytes   int64
}

func (u *uploader) send(ctx context.Context, hashes []string) error {
	var send []LocalFile
	seen := map[string]bool{}
	var bytesTotal int64
	for _, h := range hashes {
		if f, ok := u.byHash[h]; ok && !seen[h] {
			seen[h] = true
			send = append(send, f)
			bytesTotal += f.Size
		}
	}
	if len(send) == 0 {
		return nil
	}
	u.log(fmt.Sprintf("Uploading %d file(s) (%s)", len(send), humanBytes(bytesTotal)))
	for _, batch := range BatchBySize(send, UploadBatchBytes) {
		entries := make([][]byte, len(batch))
		for i, f := range batch {
			b, err := os.ReadFile(f.Source)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(b)
			if hex.EncodeToString(sum[:]) != f.Hash {
				return fmt.Errorf("%s changed during deploy; run it again", f.Path)
			}
			entries[i] = b
		}
		var gz bytes.Buffer
		zw := gzip.NewWriter(&gz)
		zw.Write(EncodeBundle(entries))
		zw.Close()
		if err := u.c.Upload(ctx, gz.Bytes()); err != nil {
			return err
		}
		if u.verbose {
			for _, f := range batch {
				u.log("   " + f.Path)
			}
		}
	}
	u.count += len(send)
	u.bytes += bytesTotal
	return nil
}

func (u *uploader) commit(ctx context.Context, body map[string]any) (*CommitResult, error) {
	body["uploadedCount"], body["uploadedBytes"] = u.count, u.bytes
	res, err := u.c.Commit(ctx, body)
	var ae *remoteclient.APIError
	// blobs can be garbage-collected by a concurrent deploy between upload and commit: upload again once
	if errors.As(err, &ae) && ae.Status == http.StatusConflict {
		if raw, ok := ae.Body["missing"].([]any); ok {
			var again []string
			for _, h := range raw {
				if s, ok := h.(string); ok {
					again = append(again, s)
				}
			}
			if err := u.send(ctx, again); err != nil {
				return nil, err
			}
			body["uploadedCount"], body["uploadedBytes"] = u.count, u.bytes
			res, err = u.c.Commit(ctx, body)
		}
	}
	return res, err
}

func detectCommit(dir string) string {
	for _, k := range []string{"GITHUB_SHA", "CI_COMMIT_SHA"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectSource() string {
	switch {
	case os.Getenv("GITHUB_ACTIONS") != "":
		return "github-actions"
	case os.Getenv("CI") != "":
		return "ci"
	}
	return "pocketone-cli"
}

func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}
