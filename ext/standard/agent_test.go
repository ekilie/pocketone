package standard_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/agent"
	"github.com/pockethost/pocketone/ext/contracts/instance/source/v1/sourcetest"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/multisite"
	"github.com/pockethost/pocketone/ext/pocker"
	"github.com/pockethost/pocketone/ext/standard"
)

type sectionJSON map[string]string

func (s sectionJSON) DecodeSection(name string, out any) (bool, error) {
	raw, ok := s[name]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal([]byte(raw), out)
}

// A host whose agent section names a model provides agent/v1 for its Pocker instances: $agent in each instance's
// pb_hooks reaches the host's model (here one that can't load, so it reports why), except in an instance whose admins
// set their own API, which answers from that API.
func TestHostProvidesAgentForPockerInstances(t *testing.T) {
	extkit.SetSectionSource(sectionJSON{"agent": `{"model": "/nonexistent/shared.gguf"}`})
	defer extkit.SetSectionSource(nil)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b, _ := io.ReadAll(r.Body); strings.Contains(string(b), `"stream":true`) {
			for _, p := range []string{"from ", "my ", "own ", "API"} {
				_, _ = io.WriteString(w, `data: {"model":"own-model","choices":[{"delta":{"content":"`+p+`"}}]}`+"\n\n")
			}
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   "own-model",
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "from my own API"}, "finish_reason": "stop"}},
		})
	}))
	defer api.Close()

	root := t.TempDir()
	probe := `routerAdd("GET", "/agent-status", (e) => e.json(200, $agent.status()))
routerAdd("GET", "/agent-stream", (e) => {
  const pieces = []
  try {
    const r = $agent.stream({ prompt: "hi" }, (t) => { pieces.push(t) })
    return e.json(200, { pieces, text: r.text, backend: r.backend })
  } catch (err) { return e.json(503, { error: String(err) }) }
})
routerAdd("GET", "/agent-complete", (e) => {
  try { return e.json(200, $agent.complete({ prompt: "hi" })) } catch (err) { return e.json(503, { error: String(err) }) }
})`
	for _, id := range []string{"a", "b"} {
		hooks := filepath.Join(root, id, "pb_hooks")
		if err := os.MkdirAll(hooks, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(hooks, "agent.pb.js"), []byte(probe), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	h := &multisite.Host{Scope: extkit.NewScope("multisite"), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := h.Use(agent.HostExtension{}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	apps := map[string]core.App{}
	m := pocker.NewManager(sourcetest.ScopeOf(pocker.Instance{ID: "a", Hosts: []string{"a.test"}}, pocker.Instance{ID: "b", Hosts: []string{"b.test"}}), pocker.ManagerOptions{
		PBRoot:   root,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Standard: standard.New(),
		Host:     h.Scope,
		Setup: func(inst pocker.Instance, app core.App) error {
			mu.Lock()
			apps[inst.ID] = app
			mu.Unlock()
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)
	defer m.Shutdown()
	for deadline := time.Now().Add(10 * time.Second); !m.Ready(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("manager not ready")
		}
	}
	get := func(url string) (int, map[string]any) {
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	// the first request starts each instance; then a's admins set their own API
	if code, st := get("http://b.test/agent-status"); code != 200 || st["backend"] != "host" || st["ready"] != false || !strings.Contains(st["message"].(string), "shared.gguf") {
		t.Fatalf("b status: %d %v", code, st)
	}
	if code, st := get("http://a.test/agent-status"); code != 200 || st["backend"] != "host" {
		t.Fatalf("a status before its own API: %d %v", code, st)
	}
	mu.Lock()
	appA := apps["a"]
	mu.Unlock()
	if err := agent.SaveSettings(appA, agent.Settings{Endpoint: api.URL + "/v1", Key: "sk", Model: "own-model"}); err != nil {
		t.Fatal(err)
	}
	if code, res := get("http://a.test/agent-complete"); code != 200 || res["backend"] != "app" || res["text"] != "from my own API" {
		t.Fatalf("a complete with its own API: %d %v", code, res)
	}
	if code, res := get("http://a.test/agent-stream"); code != 200 || res["text"] != "from my own API" || len(res["pieces"].([]any)) != 4 || res["backend"] != "app" {
		t.Fatalf("a stream with its own API: %d %v", code, res)
	}
	if code, res := get("http://b.test/agent-complete"); code != 503 || !strings.Contains(res["error"].(string), "shared.gguf") {
		t.Fatalf("b complete on the host's model: %d %v", code, res)
	}
	// agent/v1 over HTTP on the instance's own hostname needs the agent scope
	if code, _ := get("http://b.test/api/_c/agent/v1"); code != http.StatusUnauthorized && code != http.StatusForbidden {
		t.Fatalf("anonymous status over HTTP: %d", code)
	}
}
