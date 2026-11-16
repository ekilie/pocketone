package main

import (
	"bufio"
	"bytes"
	"context"
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

	"github.com/pockethost/pocketone/ext/tokens/tokenstest"
)

// End to end with the real binary: app1 is a local project, app2 a running `pocketone serve` ("prod"). app1 links to
// app2 through the browser sign-in and deploys a changed hook; app2's hooks watcher restarts it on its own (deploy
// doesn't), and app2 then serves the new hook.
//
// It builds the binary and runs real processes (a stock app restarts by replacing its process, which an in-process
// app can't show), so -short skips it.
func TestE2ELinkDeployHookRestartsRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("end to end: builds and runs the binary")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bin := filepath.Join(t.TempDir(), "pocketone")
	step(t, "build the binary")
	if out, err := exec.CommandContext(ctx, "go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	env := append(os.Environ(),
		"POCKETONE_CONFIG_DIR="+t.TempDir(), "POCKETONE_TOKEN=", "POCKETONE_TOKEN_DEFAULT=", "SSH_CONNECTION=", "SSH_TTY=")

	// app2 ("prod"): a hook answering v1, a superuser created before the first serve, then serve
	app2 := t.TempDir()
	write(t, filepath.Join(app2, "pb_hooks", "hello.pb.js"), hookSrc("v1"))
	step(t, "app2: create the superuser")
	if out, err := runBin(ctx, bin, app2, env, "superuser", "upsert", "admin@example.com", "password1234", "--dir=pb_data").CombinedOutput(); err != nil {
		t.Fatalf("superuser: %v\n%s", err, out)
	}
	addr := freeAddr(t)
	url2 := "http://" + addr
	log2 := &syncBuffer{}
	serve := runBin(ctx, bin, app2, env, "serve", "--dir=pb_data", "--http="+addr)
	serve.Stdout, serve.Stderr = log2, log2
	step(t, "app2: serve on "+addr)
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = serve.Process.Kill()
		_ = serve.Wait()
		if t.Failed() {
			t.Logf("app2 log:\n%s", log2.String())
		}
	})
	waitFor(t, ctx, "app2 answers v1", 15*time.Second, func() (bool, string) { return getBody(url2+"/hello") == "v1", getBody(url2 + "/hello") })

	// app1 (local): the same hook, changed, in a project folder
	app1 := t.TempDir()
	write(t, filepath.Join(app1, "pb_hooks", "hello.pb.js"), hookSrc("v2"))

	// link: the CLI prints the sign-in URL (--no-browser); the "browser" approves it as app2's superuser
	step(t, "app1: link to app2 (browser sign-in)")
	link := runBin(ctx, bin, app1, env, "link", url2, "--no-browser")
	stdout, err := link.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var linkOut bytes.Buffer // stdout, as the loop below reads it
	linkErr := &syncBuffer{} // stderr gets its own buffer: exec copies into it while the loop writes linkOut
	link.Stderr = linkErr
	if err := link.Start(); err != nil {
		t.Fatal(err)
	}
	browserErr := make(chan error, 1)
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := sc.Text()
		linkOut.WriteString(line + "\n")
		if u := strings.TrimSpace(line); strings.HasPrefix(u, "http") && strings.Contains(u, "/authorize?") {
			go func() {
				browserErr <- tokenstest.ApproveOrFail(u, tokenstest.Login{Collection: "_superusers", Email: "admin@example.com", Password: "password1234"})
			}()
		}
	}
	if err := link.Wait(); err != nil {
		t.Fatalf("link: %v\n%s%s", err, linkOut.String(), linkErr.String())
	}
	select {
	case err := <-browserErr:
		if err != nil {
			t.Fatalf("browser: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("link finished without the browser sign-in:\n%s", linkOut.String())
	}
	if !strings.Contains(linkOut.String(), "as admin@example.com (oauth)") {
		t.Fatalf("link output:\n%s", linkOut.String())
	}

	// deploy: the hook goes up; deploy itself doesn't restart anything
	step(t, "app1: deploy")
	out, err := runBin(ctx, bin, app1, env, "deploy", "--commit", "e2e", "-m", "e2e").CombinedOutput()
	if err != nil {
		t.Fatalf("deploy: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Deployed") || strings.Contains(string(out), "Restarted.") || strings.Contains(string(out), "restart'") {
		t.Fatalf("deploy output:\n%s", out)
	}
	if b, _ := os.ReadFile(filepath.Join(app2, "pb_hooks", "hello.pb.js")); string(b) != hookSrc("v2") {
		t.Fatalf("app2's hook on disk: %q", b)
	}

	// app2 restarts on its own (its hooks watcher) and serves the new hook
	waitFor(t, ctx, "app2 answers v2", 10*time.Second, func() (bool, string) { return getBody(url2+"/hello") == "v2", getBody(url2 + "/hello") })
	if !strings.Contains(log2.String(), "hello.pb.js changed, restarting") {
		t.Fatalf("app2 didn't log its watcher restart:\n%s", log2.String())
	}
}

func hookSrc(body string) string {
	return fmt.Sprintf("routerAdd(\"GET\", \"/hello\", (e) => e.string(200, %q))\n", body)
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
			t.Logf("  still waiting (%s): %.120s", what, got)
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
