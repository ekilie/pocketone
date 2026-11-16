package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A Pocker host from the stock binary: two instances in one process, routed by hostname. Each has its own data,
// superusers, hooks and secrets; a hooks edit or a secrets change reaches only its instance; the instances file
// applies live (block, unblock, a new instance); an unknown hostname gets 404 and no folder.
func TestE2EPockerHost(t *testing.T) {
	h := startHost(t, map[string]string{"app1": "hello from one", "app2": "hello from two"})
	app1, app2 := h.instance("app1"), h.instance("app2")

	step(t, "routing and the runtime's name")
	for _, s := range []*site{app1, app2} {
		if got := hello(s); got != s.Host+": "+h.greetings[instanceID(s)]+" | host secret: []" {
			t.Fatalf("%s /hello: %q", s.Host, got)
		}
	}
	if pb := poweredBy(t, app1); !strings.HasPrefix(pb, "Pocker ") {
		t.Fatalf("X-Powered-By: %q", pb)
	}

	step(t, "each instance has its own data and superusers")
	notesCollection(t, app1)
	addNote(t, app1, "only in one")
	if got := notes(t, app1); got != "only in one" {
		t.Fatalf("app1 notes: %q", got)
	}
	if code, b := app2.raw(http.MethodGet, "/api/collections/notes/records", app2.JWT(), nil); code != 404 {
		t.Fatalf("app2 sees app1's collection: %d %s", code, b)
	}
	if code, b := app2.raw(http.MethodGet, "/api/collections", app1.JWT(), nil); code != 401 {
		t.Fatalf("app1's superuser token on app2: %d %s", code, b)
	}

	step(t, "a hooks edit reaches only its instance")
	write(t, filepath.Join(h.Dir, "pb_root", "app1", "pb_hooks", "hello.pb.js"), helloHook("v2"))
	waitFor(t, context.Background(), "app1 serves the new hook", 20*time.Second, func() (bool, string) {
		got := hello(app1)
		return strings.HasPrefix(got, "v2 "), got
	})
	if got := hello(app2); strings.HasPrefix(got, "v2 ") {
		t.Fatalf("app2 picked up app1's hook: %q", got)
	}
	if got := notes(t, app1); got != "only in one" {
		t.Fatalf("app1 notes after its restart: %q", got)
	}
	h.wantEvents("after app1's hooks edit", map[string]int{"app1": 1, "app2": 0})

	step(t, "a secrets change reaches only its instance")
	h.greetings["app2"] = "hi again from two"
	h.writeInstances(nil)
	waitFor(t, context.Background(), "app2 sees its new secret", 20*time.Second, func() (bool, string) {
		got := hello(app2)
		return strings.Contains(got, "hi again from two"), got
	})
	if got := hello(app1); !strings.Contains(got, "hello from one") {
		t.Fatalf("app1 after app2's change: %q", got)
	}
	h.wantEvents("after app2's secrets change", map[string]int{"app1": 1, "app2": 1})

	step(t, "an unknown hostname: 404, no folder")
	if code, b := rawRequestHost(h.URL, "nobody.test", http.MethodGet, "/api/health", "", nil); code != 404 {
		t.Fatalf("unknown host: %d %s", code, b)
	}
	if _, err := os.Stat(filepath.Join(h.Dir, "pb_root", "nobody.test")); !os.IsNotExist(err) {
		t.Fatalf("a folder for the unknown host: %v", err)
	}

	step(t, "block and unblock app2 live")
	h.writeInstances(map[string]string{"app2": "    blocked: { status: 402, message: \"Payment required\" }\n"})
	waitFor(t, context.Background(), "app2 blocked", 15*time.Second, func() (bool, string) {
		code, b := app2.raw(http.MethodGet, "/api/health", "", nil)
		return code == 402 && strings.Contains(string(b), "Payment required"), fmt.Sprintf("%d %.80s", code, b)
	})
	if got := hello(app1); !strings.Contains(got, "hello from one") {
		t.Fatalf("app1 while app2 is blocked: %q", got)
	}
	h.writeInstances(nil)
	waitFor(t, context.Background(), "app2 unblocked", 15*time.Second, func() (bool, string) {
		got := hello(app2)
		return strings.Contains(got, "hi again from two"), got
	})

	step(t, "a new instance from the file, no host restart")
	h.greetings["app3"] = "hello from three"
	h.writeInstances(nil)
	app3 := h.instance("app3")
	waitFor(t, context.Background(), "app3 answers", 20*time.Second, func() (bool, string) {
		code, b := app3.raw(http.MethodGet, "/api/health", "", nil)
		return code == 200, fmt.Sprintf("%d %.80s", code, b)
	})
	if _, err := os.Stat(filepath.Join(h.Dir, "pb_root", "app3", "pb_data")); err != nil {
		t.Fatalf("app3's folder: %v", err)
	}

	step(t, "the host's health")
	code, b := rawRequestHost(h.URL, "anything.test", http.MethodGet, "/.well-known/health", "", nil)
	if code != 200 {
		t.Fatalf("/.well-known/health: %d %s", code, b)
	}
	t.Logf("health: %s", b)
	if n := strings.Count(h.Log.String(), "pocker: listening"); n != 1 {
		t.Fatalf("the host started %d times", n)
	}
}

// host is one `pocketone serve` running a Pocker host.
type host struct {
	*site
	greetings map[string]string // instance id → its GREETING secret
}

// startHost writes a host folder (pocketone.yaml, instances.yaml, one hooks file per instance), creates each
// instance's superuser before the first serve, and serves. The host's own environment has a variable no instance
// should see.
func startHost(t *testing.T, greetings map[string]string) *host {
	t.Helper()
	bin := pocketoneBin(t)
	h := &host{site: &site{t: t, Dir: t.TempDir(), Log: &syncBuffer{}}, greetings: greetings}
	addr := freeAddr(t)
	h.URL = "http://" + addr
	write(t, filepath.Join(h.Dir, "pocketone.yaml"), fmt.Sprintf(`server: { http: %q }
multisite: {}
instances: { file: ./instances.yaml }
pocker: { root: ./pb_root }
`, addr))
	h.writeInstances(nil)
	h.Env = append(baseEnv(t), "P1_E2E_HOST_SECRET=leaked")

	ctx := context.Background()
	for id := range greetings {
		write(t, filepath.Join(h.Dir, "pb_root", id, "pb_hooks", "hello.pb.js"), helloHook("v1"))
		step(t, "superuser upsert for "+id)
		if out, err := runBin(ctx, bin, h.Dir, h.Env, "superuser", "upsert", suEmail, suPassword, "--dir=pb_root/"+id+"/pb_data").CombinedOutput(); err != nil {
			t.Fatalf("superuser upsert %s: %v\n%s", id, err, out)
		}
	}
	h.cmd = runBin(ctx, bin, h.Dir, h.Env, "serve")
	h.cmd.Stdout, h.cmd.Stderr = h.Log, h.Log
	step(t, "serve the host on "+addr)
	if err := h.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- h.cmd.Wait() }()
	t.Cleanup(func() {
		_ = h.cmd.Process.Kill()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
		}
		if t.Failed() {
			t.Logf("host log:\n%s", tail(h.Log.String(), 120))
			for id := range h.greetings {
				if b, err := os.ReadFile(filepath.Join(h.Dir, "pb_root", id, "logs", "exec.log")); err == nil {
					t.Logf("%s exec.log:\n%s", id, tail(string(b), 30))
				}
			}
		}
	})
	waitFor(t, ctx, "the host answers /.well-known/health", 20*time.Second, func() (bool, string) {
		select {
		case err := <-exited:
			t.Fatalf("the host exited (%v):\n%s", err, tail(h.Log.String(), 60))
		default:
		}
		code, b := rawRequestHost(h.URL, "", http.MethodGet, "/.well-known/health", "", nil)
		return code == 200, fmt.Sprintf("%d %.80s", code, b)
	})
	for id := range greetings {
		h.instance(id).waitUp(id, 20*time.Second, exited)
	}
	return h
}

// writeInstances writes instances.yaml from h.greetings, sorted by id, with extra lines for some instances.
func (h *host) writeInstances(extra map[string]string) {
	var b strings.Builder
	b.WriteString("instances:\n")
	for _, id := range []string{"app1", "app2", "app3"} {
		g, ok := h.greetings[id]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "  - id: %s\n    hosts: [%s.test]\n    secrets: { GREETING: %q }\n%s", id, id, g, extra[id])
	}
	write(h.t, filepath.Join(h.Dir, "instances.yaml"), b.String())
}

// wantEvents checks how many times each instance restarted, from the host log's `pocker: event kind=restarted` lines.
func (h *host) wantEvents(when string, restarts map[string]int) {
	h.t.Helper()
	log := h.Log.String()
	for id, want := range restarts {
		if got := strings.Count(log, "kind=restarted id="+id+" "); got != want {
			h.t.Fatalf("%s: %s restarted %d times, want %d", when, id, got, want)
		}
	}
}

// instance is a site for one instance: the host's address with the instance's hostname.
func (h *host) instance(id string) *site {
	return &site{t: h.t, Dir: filepath.Join(h.Dir, "pb_root", id), URL: h.URL, Host: id + ".test", Env: h.Env, Log: h.Log}
}

func instanceID(s *site) string { return strings.TrimSuffix(s.Host, ".test") }

// helloHook answers GET /hello with its version, the request's host, the instance's GREETING secret and the host's
// own variable (which must be empty).
func helloHook(version string) string {
	return `routerAdd("GET", "/hello", (e) => {
	const v = "` + version + `"
	const tag = v === "v1" ? "" : v + " "
	return e.string(200, tag + e.request.host + ": " + $os.getenv("GREETING") + " | host secret: [" + $os.getenv("P1_E2E_HOST_SECRET") + "]")
})
`
}

func hello(s *site) string {
	code, b := s.raw(http.MethodGet, "/hello", "", nil)
	if code != 200 {
		return fmt.Sprintf("%d %.200s", code, b)
	}
	return string(b)
}

func poweredBy(t *testing.T, s *site) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/health", nil)
	req.Host = s.Host
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.Header.Get("X-Powered-By")
}
