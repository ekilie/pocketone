package deploy_test

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

	"github.com/pockethost/pocketone/ext/extkit/modetest"
	"github.com/pockethost/pocketone/ext/tokens/tokenstest"
)

// A local project links to a running app ("prod", a single app or an instance on a host) through the browser sign-in
// and deploys a changed hook. Deploy itself restarts nothing; prod's hooks watcher restarts it on its own (the process,
// or the instance on its host), and prod then serves the new hook.
func TestModesLinkDeploy(t *testing.T) {
	top := modetest.Topology{Instances: []modetest.Inst{{ID: "prod", Files: map[string]string{"pb_hooks/hello.pb.js": hookSrc("v1")}}}}
	modetest.Run(t, top, modetest.AllApp, func(t *testing.T, w *modetest.World) {
		prod := w.Instance("prod")
		if got := prod.Get("/hello"); got != "v1" {
			t.Fatalf("prod /hello: %q", got)
		}

		// the local project: the same hook, changed
		local := t.TempDir()
		modetest.WriteFile(t, filepath.Join(local, "pb_hooks", "hello.pb.js"), hookSrc("v2"))
		env := modetest.Env(t)
		linkWithBrowser(t, local, env, prod.URL)

		modetest.Step(t, "deploy")
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		out, err := modetest.Command(ctx, t, local, env, "deploy", "--commit", "modetest", "-m", "modetest").CombinedOutput()
		if err != nil {
			t.Fatalf("deploy: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "Deployed") || strings.Contains(string(out), "Restarted.") || strings.Contains(string(out), "restart'") {
			t.Fatalf("deploy output:\n%s", out)
		}
		if b, _ := os.ReadFile(filepath.Join(prod.Dir, "pb_hooks", "hello.pb.js")); string(b) != hookSrc("v2") {
			t.Fatalf("prod's hook on disk: %q", b)
		}

		prod.WaitRestarted("the deployed hook", 20*time.Second)
		if got := prod.Get("/hello"); got != "v2" {
			t.Fatalf("prod /hello after deploy: %q", got)
		}
	})
}

// linkWithBrowser runs `pocketone link <url> --no-browser` in dir and approves the printed sign-in URL as the
// superuser, the way a user would in a browser.
func linkWithBrowser(t *testing.T, dir string, env []string, url string) {
	t.Helper()
	modetest.Step(t, "link "+url+" (browser sign-in)")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	link := modetest.Command(ctx, t, dir, env, "link", url, "--no-browser")
	stdout, err := link.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var linkOut bytes.Buffer
	linkErr := &modetest.Buffer{} // stderr has its own buffer: exec copies into it while the loop writes linkOut
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
				browserErr <- tokenstest.ApproveOrFail(u, tokenstest.Login{Collection: "_superusers", Email: modetest.SuperuserEmail, Password: modetest.SuperuserPassword})
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
	if !strings.Contains(linkOut.String(), "as "+modetest.SuperuserEmail+" (oauth)") {
		t.Fatalf("link output:\n%s", linkOut.String())
	}
}

func hookSrc(body string) string {
	return fmt.Sprintf("routerAdd(\"GET\", \"/hello\", (e) => e.string(200, %q))\n", body)
}
