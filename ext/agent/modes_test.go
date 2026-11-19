package agent_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pockethost/pocketone/ext/extkit/modetest"
)

// probe is pb_hooks code that calls $agent the way an app does, and reports what it got.
const probe = `routerAdd("GET", "/probe/status", (e) => e.json(200, $agent.status()))
routerAdd("GET", "/probe/complete", (e) => {
  try { return e.json(200, $agent.complete({ prompt: "hi" })) } catch (err) { return e.json(503, { error: String(err) }) }
})
routerAdd("GET", "/probe/stream", (e) => {
  const pieces = []
  try {
    const r = $agent.stream({ prompt: "hi" }, (t) => { pieces.push(t) })
    return e.json(200, { pieces, text: r.text, backend: r.backend })
  } catch (err) { return e.json(503, { error: String(err) }) }
})
routerAdd("GET", "/probe/prims", async (e) => {
  const w = $agent.warm({ system: "You help." })
  const w2 = await $agent.warm.async({ system: "You help.", wait: "1s" })
  let text = "", error = ""
  try { text = (await $agent.complete.async({ prompt: "hi", wait: "1s" })).text } catch (err) { error = String(err) }
  return e.json(200, { id: w.id, same: w.id === w2.id, warmed: w.warmed, message: w.message, forget: typeof w.forget,
    count: $agent.count("one two three four"), threads: $agent.threads.list(), text, error })
})
`

// fakeOpenAI is an OpenAI-compatible chat completions API, streaming or not.
func fakeOpenAI(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), `"stream":true`) {
			for _, p := range []string{"from ", "the ", "API"} {
				_, _ = io.WriteString(w, `data: {"model":"fake","choices":[{"delta":{"content":"`+p+`"}}]}`+"\n\n")
			}
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "fake",
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "from the API"}, "finish_reason": "stop"}}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Settings → LLM and $agent on the real binary, as a single app and as an instance on a host whose agent section names
// a model (one that can't load, so it reports why). The app chooses its own API, then off, over the settings API, and
// $agent and agent/v1 (complete and stream, in pb_hooks and over HTTP) follow; on the host, built in is refused and
// the host's model is the default.
func TestModesLLMSettings(t *testing.T) {
	api := fakeOpenAI(t)
	top := modetest.Topology{
		Instances: []modetest.Inst{{ID: "app", Files: map[string]string{"pb_hooks/probe.pb.js": probe}}},
		Hosts: []modetest.HostSpec{{ID: "host", Config: map[string]any{
			"agent":  map[string]any{"model": "/nonexistent/shared.gguf"},
			"pocker": map[string]any{"root": "./pb_root", "egress_allow": []string{"127.0.0.1/32"}}, // the fake API
		}}},
	}
	modetest.Run(t, top, modetest.AllApp, func(t *testing.T, w *modetest.World) {
		s := w.Instance("app")
		su := s.JWT()
		hosted := w.Mode == modetest.Host

		var view struct {
			Values struct {
				Effective, Source, YAMLModel string
				Hosted                       bool
				Models                       []any
			} `json:"values"`
		}
		s.MustJSON(http.MethodGet, "/api/_c/settings/v1/get?extension=agent", su, nil, 200, &view)
		v := view.Values
		if hosted {
			// the host's model answers by default; its file is missing, so the status says so
			if !v.Hosted || v.Effective != "host" || v.Source != "host" || v.YAMLModel != "/nonexistent/shared.gguf" || len(v.Models) != 0 {
				t.Fatalf("hosted settings: %+v", v)
			}
			if b := s.Get("/probe/status"); !strings.Contains(b, `"backend":"host"`) || !strings.Contains(b, "shared.gguf") {
				t.Fatalf("hosted status: %s", b)
			}
			code, b := s.Raw(http.MethodPost, "/api/_c/settings/v1/save", su, map[string]any{"extension": "agent", "values": map[string]any{"mode": "builtin", "model": "qwen3-0.6b"}})
			if code != 400 || !strings.Contains(string(b), "host decides") {
				t.Fatalf("built in under a host: %d %s", code, b)
			}
		} else if v.Hosted || v.Effective != "off" || v.Source != "none" || len(v.Models) == 0 {
			t.Fatalf("single app settings: %+v", v)
		}

		// the app's own API
		s.MustJSON(http.MethodPost, "/api/_c/settings/v1/save", su, map[string]any{"extension": "agent",
			"values": map[string]any{"mode": "api", "endpoint": api.URL + "/v1", "key": "sk", "apiModel": "fake"}}, 200, nil)
		if b := s.Get("/probe/complete"); !strings.Contains(b, `"text":"from the API"`) || !strings.Contains(b, `"backend":"app"`) {
			t.Fatalf("complete with the app's API: %s", b)
		}
		if b := s.Get("/probe/stream"); !strings.Contains(b, `"pieces":["from ","the ","API"]`) {
			t.Fatalf("stream with the app's API: %s", b)
		}
		// the primitives from JS: warm (same id for the same text; not warmed: the API caches prompts itself), count
		// (estimated), threads, and complete.async
		var prims struct {
			ID, Message, Forget, Text, Error string
			Same, Warmed                     bool
			Count                            int
			Threads                          []any
		}
		if err := json.Unmarshal([]byte(s.Get("/probe/prims")), &prims); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(prims.ID, "w_") || !prims.Same || prims.Warmed || !strings.Contains(prims.Message, "own API") ||
			prims.Forget != "function" || prims.Count <= 0 || prims.Threads == nil || prims.Text != "from the API" {
			t.Fatalf("primitives from JS: %+v", prims)
		}
		code, b := s.Raw(http.MethodPost, "/api/_c/agent/v1/stream", s.Token("agent"), map[string]any{"prompt": "hi"})
		if code != 200 || strings.Count(string(b), "event: stream") != 4 || !strings.Contains(string(b), `"done":true`) {
			t.Fatalf("agent/v1 stream over HTTP: %d %s", code, b)
		}
		if code, _ := s.Raw(http.MethodPost, "/api/_c/agent/v1/complete", "", map[string]any{"prompt": "hi"}); code != 401 && code != 403 {
			t.Fatalf("anonymous complete: %d", code)
		}

		// off
		s.MustJSON(http.MethodPost, "/api/_c/settings/v1/save", su, map[string]any{"extension": "agent", "values": map[string]any{"mode": "off"}}, 200, nil)
		if b := s.Get("/probe/complete"); !strings.Contains(b, "Settings → LLM") {
			t.Fatalf("complete when off: %s", b)
		}
	})
}
