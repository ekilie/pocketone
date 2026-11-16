package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pockethost/pocketone/ext/tokens/tokenstest"
)

// End to end with the real binary: app1 is a local project, app2 a running `pocketone serve` ("prod"). app1 links to
// app2 through the browser sign-in and deploys a changed hook; app2's hooks watcher restarts it on its own (deploy
// doesn't), and app2 then serves the new hook.
//
// It runs real processes because a stock app restarts by replacing its process, which an in-process app can't show.
func TestE2ELinkDeployHookRestartsRemote(t *testing.T) {
	app2 := startSite(t, siteOptions{Files: map[string]string{"pb_hooks/hello.pb.js": hookSrc("v1")}})
	waitFor(t, context.Background(), "app2 answers v1", 5*time.Second, func() (bool, string) {
		return getBody(app2.URL+"/hello") == "v1", getBody(app2.URL + "/hello")
	})

	// app1 (local): the same hook, changed, in a project folder
	app1 := t.TempDir()
	write(t, filepath.Join(app1, "pb_hooks", "hello.pb.js"), hookSrc("v2"))
	env := baseEnv(t)
	linkWithBrowser(t, app1, env, app2.URL)

	// deploy: the hook goes up; deploy itself doesn't restart anything
	step(t, "app1: deploy")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	out, err := runBin(ctx, pocketoneBin(t), app1, env, "deploy", "--commit", "e2e", "-m", "e2e").CombinedOutput()
	if err != nil {
		t.Fatalf("deploy: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Deployed") || strings.Contains(string(out), "Restarted.") || strings.Contains(string(out), "restart'") {
		t.Fatalf("deploy output:\n%s", out)
	}
	if b, _ := os.ReadFile(filepath.Join(app2.Dir, "pb_hooks", "hello.pb.js")); string(b) != hookSrc("v2") {
		t.Fatalf("app2's hook on disk: %q", b)
	}

	// app2 restarts on its own (its hooks watcher) and serves the new hook
	waitFor(t, ctx, "app2 answers v2", 10*time.Second, func() (bool, string) {
		return getBody(app2.URL+"/hello") == "v2", getBody(app2.URL + "/hello")
	})
	if !strings.Contains(app2.Log.String(), "hello.pb.js changed, restarting") {
		t.Fatalf("app2 didn't log its watcher restart:\n%s", app2.Log.String())
	}
}

// linkWithBrowser runs `pocketone link <url> --no-browser` in dir and approves the printed sign-in URL as the
// superuser, the way a user would in a browser.
func linkWithBrowser(t *testing.T, dir string, env []string, url string) {
	t.Helper()
	step(t, "link "+url+" (browser sign-in)")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	link := runBin(ctx, pocketoneBin(t), dir, env, "link", url, "--no-browser")
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
				browserErr <- tokenstest.ApproveOrFail(u, tokenstest.Login{Collection: "_superusers", Email: suEmail, Password: suPassword})
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
	if !strings.Contains(linkOut.String(), "as "+suEmail+" (oauth)") {
		t.Fatalf("link output:\n%s", linkOut.String())
	}
}

func hookSrc(body string) string {
	return fmt.Sprintf("routerAdd(\"GET\", \"/hello\", (e) => e.string(200, %q))\n", body)
}
