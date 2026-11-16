package main

// The end-to-end harness: the real pocketone binary, built once per test run, serving real apps in temp folders.
// Tests talk to them over HTTP like any client would. -short skips every test that uses it.
//
// The binary lives under the temp folder, so it runs in dev mode (osutils.IsProbablyGoRun), which turns the dev mail
// trap on; pass --dev=false for production behavior. Every wait is bounded and logged, so a stuck run shows where.

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	suEmail    = "admin@example.com"
	suPassword = "password1234"
)

var (
	binOnce sync.Once
	binPath string
	binErr  error
	binDir  string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if binDir != "" {
		_ = os.RemoveAll(binDir)
	}
	os.Exit(code)
}

// pocketoneBin builds the binary on first use (about 2s with a warm cache).
func pocketoneBin(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("end to end: builds and runs the binary")
	}
	binOnce.Do(func() {
		binDir, binErr = os.MkdirTemp("", "pocketone-e2e-")
		if binErr != nil {
			return
		}
		binPath = filepath.Join(binDir, "pocketone")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		start := time.Now()
		out, err := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".").CombinedOutput()
		if err != nil {
			binErr = fmt.Errorf("build: %v\n%s", err, out)
		}
		fmt.Fprintf(os.Stderr, "e2e: built %s in %s\n", binPath, time.Since(start).Round(time.Millisecond))
	})
	if binErr != nil {
		t.Fatal(binErr)
	}
	return binPath
}

// baseEnv is the environment for every process: its own config folder (saved logins), no tokens from the
// developer's shell, never "over SSH".
func baseEnv(t *testing.T) []string {
	return append(os.Environ(),
		"P1_CONFIG_DIR="+t.TempDir(), "P1_TOKEN=", "P1_TOKEN_DEFAULT=", "SSH_CONNECTION=", "SSH_TTY=")
}

// siteOptions describes one app folder and how to serve it.
type siteOptions struct {
	Files map[string]string // relative path → content (pb_hooks/x.pb.js, pb_pages/index.ejs, pocketone.yaml, ...)
	Args  []string          // extra serve flags
	Env   []string          // extra environment
}

// site is one running `pocketone serve`.
type site struct {
	t       *testing.T
	Dir     string // the app folder (pb_data, pb_hooks, ... inside)
	URL     string
	TrapURL string // the dev mail trap's inbox (dev mode only)
	Env     []string
	Log     *syncBuffer
	cmd     *exec.Cmd
	jwt     string
	starts  int // "Server started" lines seen when last checked
}

// startSite writes opts.Files, creates the superuser before the first serve, serves on a free port and waits until
// /api/health answers.
func startSite(t *testing.T, opts siteOptions) *site {
	t.Helper()
	bin := pocketoneBin(t)
	s := &site{t: t, Dir: t.TempDir(), Log: &syncBuffer{}}
	for rel, content := range opts.Files {
		write(t, filepath.Join(s.Dir, rel), content)
	}
	trapHTTP, trapSMTP := freeAddr(t), freeAddr(t)
	s.TrapURL = "http://" + trapHTTP
	s.Env = append(baseEnv(t), "P1_MAILTRAP_HTTP="+trapHTTP, "P1_MAILTRAP_SMTP="+trapSMTP)
	s.Env = append(s.Env, opts.Env...)

	ctx := context.Background()
	step(t, "superuser upsert")
	if out, err := runBin(ctx, bin, s.Dir, s.Env, "superuser", "upsert", suEmail, suPassword, "--dir=pb_data").CombinedOutput(); err != nil {
		t.Fatalf("superuser upsert: %v\n%s", err, out)
	}
	addr := freeAddr(t)
	s.URL = "http://" + addr
	args := append([]string{"serve", "--dir=pb_data", "--http=" + addr}, opts.Args...)
	s.cmd = runBin(ctx, bin, s.Dir, s.Env, args...)
	s.cmd.Stdout, s.cmd.Stderr = s.Log, s.Log
	step(t, "serve "+strings.Join(args[1:], " "))
	if err := s.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- s.cmd.Wait() }()
	t.Cleanup(func() {
		_ = s.cmd.Process.Kill()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
		}
		if t.Failed() {
			t.Logf("site log (%s):\n%s", s.URL, tail(s.Log.String(), 120))
		}
	})
	s.waitUp("serve", 20*time.Second, exited)
	s.starts = s.serverStarts()
	return s
}

// waitUp waits for /api/health, failing at once if the process exits.
func (s *site) waitUp(what string, limit time.Duration, exited <-chan error) {
	s.t.Helper()
	step(s.t, "wait: "+what+" answers /api/health")
	deadline := time.Now().Add(limit)
	for {
		if exited != nil {
			select {
			case err := <-exited:
				s.t.Fatalf("%s: the process exited (%v):\n%s", what, err, tail(s.Log.String(), 60))
			default:
			}
		}
		if code, _ := s.raw(http.MethodGet, "/api/health", "", nil); code == 200 {
			step(s.t, "ready: "+what)
			return
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("gave up after %s waiting for %s:\n%s", limit, what, tail(s.Log.String(), 60))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *site) serverStarts() int { return strings.Count(s.Log.String(), "Server started at") }

// waitRestarted waits until the process has started its server again (an execve restart keeps the PID, so the log
// is the signal) and answers /api/health. Superuser JWTs survive; nothing to refresh.
func (s *site) waitRestarted(why string, limit time.Duration) {
	s.t.Helper()
	waitFor(s.t, context.Background(), why+": server started again", limit, func() (bool, string) {
		n := s.serverStarts()
		return n > s.starts, fmt.Sprintf("%d server starts", n)
	})
	s.waitUp(why, limit, nil)
	s.starts = s.serverStarts()
}

// JWT is a superuser auth token (cached).
func (s *site) JWT() string {
	s.t.Helper()
	if s.jwt == "" {
		var out struct {
			Token string `json:"token"`
		}
		s.mustJSON(http.MethodPost, "/api/collections/_superusers/auth-with-password", "",
			map[string]string{"identity": suEmail, "password": suPassword}, 200, &out)
		s.jwt = out.Token
	}
	return s.jwt
}

// Token creates an API token for the superuser (no scopes: the owner's full rights).
func (s *site) Token(scopes ...string) string {
	s.t.Helper()
	if scopes == nil {
		scopes = []string{}
	}
	var out struct {
		Token string `json:"token"`
	}
	s.mustJSON(http.MethodPost, "/api/_c/tokens/v1", s.JWT(), map[string]any{"name": "e2e", "scopes": scopes}, 200, &out)
	if !strings.HasPrefix(out.Token, "pbt_") {
		s.t.Fatalf("token: %q", out.Token)
	}
	return out.Token
}

// raw sends one request (body: nil, a string, or anything JSON-encodable) and returns status and body.
func (s *site) raw(method, path, auth string, body any) (int, []byte) {
	return rawRequest(s.URL, method, path, auth, body)
}

func rawRequest(base, method, path, auth string, body any) (int, []byte) {
	var r io.Reader
	ctype := ""
	switch b := body.(type) {
	case nil:
	case string:
		r, ctype = strings.NewReader(b), "application/json"
	default:
		j, _ := json.Marshal(b)
		r, ctype = bytes.NewReader(j), "application/json"
	}
	req, err := http.NewRequest(method, base+path, r)
	if err != nil {
		return 0, []byte(err.Error())
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return 0, []byte(err.Error())
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

// mustJSON sends a request, fails unless the status is want, and decodes the answer into out (if not nil).
func (s *site) mustJSON(method, path, auth string, body any, want int, out any) []byte {
	s.t.Helper()
	code, b := s.raw(method, path, auth, body)
	if code != want {
		s.t.Fatalf("%s %s: %d, want %d\n%s", method, path, code, want, b)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			s.t.Fatalf("%s %s: decode: %v\n%s", method, path, err, b)
		}
	}
	return b
}

// run runs a pocketone command in the site's folder (with its environment) and returns its output.
func (s *site) run(args ...string) (string, error) {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := runBin(ctx, pocketoneBin(s.t), s.Dir, s.Env, args...).CombinedOutput()
	return string(out), err
}

func tail(s string, lines int) string {
	parts := strings.Split(s, "\n")
	if len(parts) > lines {
		parts = append([]string{fmt.Sprintf("... (%d lines before)", len(parts)-lines)}, parts[len(parts)-lines:]...)
	}
	return strings.Join(parts, "\n")
}

func step(t *testing.T, what string) { t.Logf("%s %s", time.Now().Format("15:04:05.000"), what) }

func runBin(ctx context.Context, bin, dir string, env []string, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, bin, args...)
	c.Dir, c.Env, c.Stdin = dir, env, nil
	c.WaitDelay = 5 * time.Second
	return c
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func getBody(u string) string {
	c := &http.Client{Timeout: 2 * time.Second}
	res, err := c.Get(u)
	if err != nil {
		return "error: " + err.Error()
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return strings.TrimSpace(string(b))
}

// waitFor polls ok every 200ms, logging each new answer, and fails after limit.
func waitFor(t *testing.T, ctx context.Context, what string, limit time.Duration, ok func() (bool, string)) {
	t.Helper()
	step(t, "wait: "+what)
	deadline := time.Now().Add(limit)
	last := ""
	for {
		good, got := ok()
		if good {
			step(t, "ready: "+what)
			return
		}
		if got != last {
			t.Logf("  still waiting (%s): %.200s", what, got)
			last = got
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			t.Fatalf("gave up after %s waiting for %s; last answer: %s", limit, what, got)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// syncBuffer is a bytes.Buffer safe for a process's output and the test reading it at once.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func errorAs(err error, target any) bool { return errors.As(err, target) }

// zipHas reports whether the zip at path has an entry starting with prefix.
func zipHas(t *testing.T, path, prefix string) bool {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, prefix) {
			return true
		}
	}
	return false
}
