package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	remotev1 "pocketone.dev/ext/contracts/remote/v1"
	"pocketone.dev/ext/extkit"
	"strings"
	"time"

	remoteclient "pocketone.dev/ext/contracts/remote/v1/remoteclient"
)

// FileEntry is one entry of a remote listing.
type FileEntry struct {
	Path        string `json:"path"`
	IsDirectory bool   `json:"isDirectory"`
	Size        int64  `json:"size"`
	Mtime       int64  `json:"mtime"`
}

// ListFiles lists a remote folder (recursive: the whole subtree).
func (c *Client) ListFiles(ctx context.Context, path string, recursive bool) ([]FileEntry, bool, error) {
	q := url.Values{"path": {path}}
	if recursive {
		q.Set("recursive", "1")
	}
	var res struct {
		Entries   []FileEntry `json:"entries"`
		Truncated bool        `json:"truncated"`
	}
	err := c.call(ctx, http.MethodGet, "/files?"+q.Encode(), nil, &res)
	return res.Entries, res.Truncated, err
}

// Download writes a remote file to dest (via a temp file, then rename).
func (c *Client) Download(ctx context.Context, path, dest string) error {
	req, err := c.NewRequest(ctx, http.MethodGet, c.Prefix+"/files/raw?"+url.Values{"path": {path}}.Encode(), nil)
	if err != nil {
		return err
	}
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return remoteclient.DecodeResponse(res, nil)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".pull-tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, res.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// DefaultPullPaths are pulled when `pull` names none: the code set's folders and root files.
var DefaultPullPaths = []string{"pb_hooks", "pb_migrations", "pb_pages", "pb_public", "pb_templates", "package.json"}

// PullOptions configure [Pull].
type PullOptions struct {
	Dir    string   // local project root
	Paths  []string // remote paths (default DefaultPullPaths)
	DryRun bool
	Log    func(string)
}

// Pull downloads remote files into the project, keeping their paths. pb_data is only pulled when asked for (and
// the remote refuses while the app runs).
func Pull(ctx context.Context, c *Client, opts PullOptions) (int, error) {
	log := opts.Log
	if log == nil {
		log = func(string) {}
	}
	paths := opts.Paths
	explicit := len(paths) > 0
	if !explicit {
		paths = DefaultPullPaths
	}
	n := 0
	for _, p := range paths {
		p = strings.Trim(p, "/")
		entries, truncated, err := c.ListFiles(ctx, p, true)
		if err != nil {
			if !explicit && remoteclient.IsStatus(err, http.StatusNotFound) {
				continue // a default folder the remote doesn't have
			}
			return n, fmt.Errorf("%s: %w", p, err)
		}
		if truncated {
			log(fmt.Sprintf("%s: the listing was truncated; pull subfolders separately", p))
		}
		for _, e := range entries {
			if e.IsDirectory {
				continue
			}
			if !explicit && (e.Path == "pb_data" || strings.HasPrefix(e.Path, "pb_data/")) {
				continue
			}
			dest := filepath.Join(opts.Dir, filepath.FromSlash(e.Path))
			if !insideDir(opts.Dir, dest) {
				return n, fmt.Errorf("refusing remote path %q", e.Path)
			}
			if opts.DryRun {
				log("   " + e.Path)
				n++
				continue
			}
			if err := c.Download(ctx, e.Path, dest); err != nil {
				return n, fmt.Errorf("%s: %w", e.Path, err)
			}
			if e.Mtime > 0 {
				t := time.UnixMilli(e.Mtime)
				_ = os.Chtimes(dest, t, t)
			}
			n++
		}
	}
	return n, nil
}

func insideDir(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// LogEntry is one line of the app's log stream.
type LogEntry struct {
	Message string `json:"message"`
	Stream  string `json:"stream"`
	Time    string `json:"time"`
}

// StreamLogs replays the last n log entries, then follows new ones until ctx ends or the stream closes. It reads
// remote/v1's log stream.
func (c *Client) StreamLogs(ctx context.Context, n int, fn func(LogEntry)) error {
	st, err := c.remote().Logs(ctx, extkit.Auth{}, remotev1.LogsInput{N: n})
	if err != nil {
		return err
	}
	defer st.Close()
	emit := func() {
		for _, raw := range st.Poll() {
			m, _ := raw.(map[string]any)
			e := LogEntry{Message: fmt.Sprint(m["message"]), Time: fmt.Sprint(m["time"])}
			e.Stream, _ = m["stream"].(string)
			if lvl, _ := m["level"].(string); e.Stream == "" && (lvl == "error" || lvl == "warn") {
				e.Stream = "stderr"
			}
			fn(e)
		}
	}
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-st.Done:
			emit()
			return errors.New("the log stream ended")
		case <-tick.C:
			emit()
		}
	}
}

// remote is remote/v1 on this client's remote, with its login.
func (c *Client) remote() *remotev1.Client {
	tok := ""
	if c.Token != "" {
		tok = "Bearer " + c.Token
	}
	return remotev1.NewClient(c.Remote.URL, tok, c.HTTP)
}
