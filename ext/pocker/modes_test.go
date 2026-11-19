package pocker_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pocketone.dev/ext/extkit/modetest"
)

// helloHook answers GET /hello with its version, the instance's GREETING secret and the host's own variable (which
// must be empty), and logs lifecycle lines, so a test can see one app stop before the next starts.
func helloHook(version string) string {
	return `routerAdd("GET", "/hello", (e) => {
	const v = "` + version + `"
	const tag = v === "v1" ? "" : v + " "
	return e.string(200, tag + $os.getenv("GREETING") + " | host secret: [" + $os.getenv("P1_MODETEST_HOST_SECRET") + "]")
})

onBootstrap((e) => {
	e.next()
	console.log("lifecycle: up " + $os.getenv("GREETING"))
})
onTerminate((e) => {
	console.log("lifecycle: down " + $os.getenv("GREETING"))
	e.next()
})
`
}

func inst(id, greeting string) modetest.Inst {
	return modetest.Inst{ID: id, Secrets: map[string]string{"GREETING": greeting},
		Files: map[string]string{"pb_hooks/hello.pb.js": helloHook("v1")}}
}

// The host's own environment has a variable no instance may see.
var hostEnv = []string{"P1_MODETEST_HOST_SECRET=leaked"}

// A Pocker host from the stock binary: two instances in one process, routed by hostname. Each has its own data,
// superusers, hooks and secrets; a hooks edit or a secrets change reaches only its instance; the instances file
// applies live (block, unblock, a new instance); an unknown hostname gets 404 and no folder.
func TestModesHost(t *testing.T) {
	top := modetest.Topology{Instances: []modetest.Inst{inst("app1", "hello from one"), inst("app2", "hello from two")}, Env: hostEnv}
	modetest.Run(t, top, []modetest.Mode{modetest.Host}, func(t *testing.T, w *modetest.World) {
		app1, app2 := w.Instance("app1"), w.Instance("app2")
		host := w.Host("host")

		modetest.Step(t, "routing, secrets, and the runtime's name")
		if got := app1.Get("/hello"); got != "hello from one | host secret: []" {
			t.Fatalf("app1 /hello: %q", got)
		}
		if got := app2.Get("/hello"); got != "hello from two | host secret: []" {
			t.Fatalf("app2 /hello: %q", got)
		}
		res, err := app1.Client().Get(app1.URL + "/api/health")
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if pb := res.Header.Get("X-Powered-By"); !strings.HasPrefix(pb, "Pocker ") {
			t.Fatalf("X-Powered-By: %q", pb)
		}

		modetest.Step(t, "each instance has its own data and superusers")
		notesCollection(t, app1)
		app1.MustJSON(http.MethodPost, "/api/collections/notes/records", app1.JWT(), map[string]string{"text": "only in one"}, 200, nil)
		if got := notes(t, app1); got != "only in one" {
			t.Fatalf("app1 notes: %q", got)
		}
		if code, b := app2.Raw(http.MethodGet, "/api/collections/notes/records", app2.JWT(), nil); code != 404 {
			t.Fatalf("app2 sees app1's collection: %d %s", code, b)
		}
		if code, b := app2.Raw(http.MethodGet, "/api/collections", app1.JWT(), nil); code != 401 {
			t.Fatalf("app1's superuser token on app2: %d %s", code, b)
		}

		modetest.Step(t, "a hooks edit reaches only its instance")
		app1.Write("pb_hooks/hello.pb.js", helloHook("v2"))
		modetest.WaitFor(t, "app1 serves the new hook", 20*time.Second, func() (bool, string) {
			got := app1.Get("/hello")
			return strings.HasPrefix(got, "v2 "), got
		})
		if got := app2.Get("/hello"); strings.HasPrefix(got, "v2 ") {
			t.Fatalf("app2 picked up app1's hook: %q", got)
		}
		if got := notes(t, app1); got != "only in one" {
			t.Fatalf("app1 notes after its restart: %q", got)
		}
		wantRestarts(t, "after app1's hooks edit", map[*modetest.Site]int{app1: 1, app2: 0})

		modetest.Step(t, "a secrets change reaches only its instance")
		w.Update("app2", func(in *modetest.Inst) { in.Secrets["GREETING"] = "hi again from two" })
		modetest.WaitFor(t, "app2 sees its new secret", 20*time.Second, func() (bool, string) {
			got := app2.Get("/hello")
			return strings.Contains(got, "hi again from two"), got
		})
		if got := app1.Get("/hello"); !strings.Contains(got, "hello from one") {
			t.Fatalf("app1 after app2's change: %q", got)
		}
		wantRestarts(t, "after app2's secrets change", map[*modetest.Site]int{app1: 1, app2: 1})

		modetest.Step(t, "an unknown hostname: 404, no folder")
		if code, b := app1.Raw(http.MethodGet, strings.Replace(app1.URL, "app1.", "nobody.", 1)+"/api/health", "", nil); code != 404 {
			t.Fatalf("unknown host: %d %s", code, b)
		}
		if _, err := os.Stat(filepath.Join(host.Dir, "pb_root", "nobody")); !os.IsNotExist(err) {
			t.Fatalf("a folder for the unknown host: %v", err)
		}

		modetest.Step(t, "block and unblock app2 live")
		w.Update("app2", func(in *modetest.Inst) {
			in.HostFields = map[string]any{"blocked": map[string]any{"status": 402, "message": "Payment required"}}
		})
		modetest.WaitFor(t, "app2 blocked", 15*time.Second, func() (bool, string) {
			code, b := app2.Raw(http.MethodGet, "/api/health", "", nil)
			return code == 402 && strings.Contains(string(b), "Payment required"), fmt.Sprintf("%d %.80s", code, b)
		})
		if got := app1.Get("/hello"); !strings.Contains(got, "hello from one") {
			t.Fatalf("app1 while app2 is blocked: %q", got)
		}
		w.Update("app2", func(in *modetest.Inst) { in.HostFields = nil })
		modetest.WaitFor(t, "app2 unblocked", 15*time.Second, func() (bool, string) {
			got := app2.Get("/hello")
			return strings.Contains(got, "hi again from two"), got
		})

		modetest.Step(t, "a new instance from the file, no host restart")
		app3 := w.Add(inst("app3", "hello from three"))
		if got := app3.Get("/hello"); got != "hello from three | host secret: []" {
			t.Fatalf("app3 /hello: %q", got)
		}
		if _, err := os.Stat(filepath.Join(app3.Dir, "pb_data")); err != nil {
			t.Fatalf("app3's folder: %v", err)
		}

		modetest.Step(t, "the host's health")
		code, b := app1.Raw(http.MethodGet, "/.well-known/health", "", nil)
		if code != 200 {
			t.Fatalf("/.well-known/health: %d %s", code, b)
		}
		if n := strings.Count(host.Log.String(), "pocker: listening"); n != 1 {
			t.Fatalf("the host started %d times", n)
		}
	})
}

// A settings change (here a secret) restarts the instance the way a process restarts: the running app stops before
// the new one starts, so two apps never have the instance's folder open at once.
func TestModesSettingsChangeStopsFirst(t *testing.T) {
	top := modetest.Topology{Instances: []modetest.Inst{inst("app1", "one")}}
	modetest.Run(t, top, []modetest.Mode{modetest.Host}, func(t *testing.T, w *modetest.World) {
		app1 := w.Instance("app1")
		if got := app1.Get("/hello"); !strings.HasPrefix(got, "one |") {
			t.Fatalf("/hello: %q", got)
		}
		w.Update("app1", func(in *modetest.Inst) { in.Secrets["GREETING"] = "two" })
		modetest.WaitFor(t, "app1 serves its new secret", 20*time.Second, func() (bool, string) {
			got := app1.Get("/hello")
			return strings.HasPrefix(got, "two |"), got
		})
		var lines []string
		modetest.WaitFor(t, "the old app's down line", 15*time.Second, func() (bool, string) {
			lines = lifecycleLines(app1.Dir)
			return indexOf(lines, "down one") >= 0, strings.Join(lines, ", ")
		})
		down, up := indexOf(lines, "down one"), indexOf(lines, "up two")
		if up < 0 || down > up {
			t.Fatalf("the new app started before the old one stopped: %s", strings.Join(lines, ", "))
		}
	})
}

func wantRestarts(t *testing.T, when string, want map[*modetest.Site]int) {
	t.Helper()
	for s, n := range want {
		if got := s.Restarts(); got != n {
			t.Fatalf("%s: %s restarted %d times, want %d", when, s.ID, got, n)
		}
	}
}

func notesCollection(t *testing.T, s *modetest.Site) {
	t.Helper()
	s.MustJSON(http.MethodPost, "/api/collections", s.JWT(), map[string]any{
		"name": "notes", "type": "base", "fields": []map[string]any{{"name": "text", "type": "text"}},
	}, 200, nil)
}

func notes(t *testing.T, s *modetest.Site) string {
	t.Helper()
	var out struct {
		Items []struct {
			Text string `json:"text"`
		} `json:"items"`
	}
	s.MustJSON(http.MethodGet, "/api/collections/notes/records?perPage=200", s.JWT(), nil, 200, &out)
	var texts []string
	for _, it := range out.Items {
		texts = append(texts, it.Text)
	}
	return strings.Join(texts, ",")
}

// lifecycleLines reads the "lifecycle: ..." messages from an instance's exec.log, in order, without the prefix.
func lifecycleLines(instDir string) []string {
	b, err := os.ReadFile(filepath.Join(instDir, "logs", "exec.log"))
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		var rec struct {
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(line), &rec) == nil {
			if m, ok := strings.CutPrefix(strings.TrimSpace(rec.Message), "lifecycle: "); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}
