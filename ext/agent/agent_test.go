package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"

	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	settingsv1 "github.com/pockethost/pocketone/ext/contracts/settings/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// serve fires the app's OnServe hooks, as `serve` does (the runner starts there).
func serve(t *testing.T, app core.App) {
	t.Helper()
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.OnServe().Trigger(&core.ServeEvent{App: app, Router: r}, func(*core.ServeEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func testApp(t *testing.T) core.App {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	return app
}

// jsonSource is pocketone.yaml for tests: section name → JSON.
type jsonSource map[string]string

func (s jsonSource) DecodeSection(name string, out any) (bool, error) {
	raw, ok := s[name]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal([]byte(raw), out)
}

func withSection(t *testing.T, raw string) {
	t.Helper()
	extkit.SetSectionSource(jsonSource{Name: raw})
	t.Cleanup(func() { extkit.SetSectionSource(nil) })
}

// fakeModel answers every prompt with its reply, counting calls.
type fakeModel struct {
	reply   string
	calls   atomic.Int32
	lastMax atomic.Int32 // the MaxTokens of the last call
	prompt  atomic.Value
	delay   time.Duration
}

func (f *fakeModel) chatPrompt(msgs []agentv1.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Role + ": " + m.Content + "\n")
	}
	return b.String()
}

func (f *fakeModel) generate(ctx context.Context, prompt string, o genOptions) (genResult, error) {
	f.calls.Add(1)
	f.lastMax.Store(int32(o.MaxTokens))
	f.prompt.Store(prompt)
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return genResult{StopReason: "cancelled"}, ctx.Err()
		}
	}
	if o.OnText != nil { // stream the reply a word at a time, as the engine streams tokens
		var sent strings.Builder
		for _, w := range strings.SplitAfter(f.reply, " ") {
			sent.WriteString(w)
			if !o.OnText(w) {
				return genResult{Text: sent.String(), OutputTokens: 1, StopReason: "cancelled"}, nil
			}
		}
	}
	return genResult{Text: f.reply, InputTokens: len(prompt) / 4, OutputTokens: 7, StopReason: "stop"}, nil
}
func (f *fakeModel) name() string { return "fake" }
func (f *fakeModel) close() error { return nil }

// useFake makes the runner load fake for any model file, and gives each test its own runner cache.
func useFake(t *testing.T, fake *fakeModel) string {
	t.Helper()
	prev := loadModel
	loadModel = func(string, int) (localModel, error) { return fake, nil }
	runnersMu.Lock()
	runners = map[string]*runner{}
	runnersMu.Unlock()
	t.Cleanup(func() {
		runnersMu.Lock()
		for _, r := range runners { // background loads still reading loadModel finish first
			<-r.ready
		}
		runnersMu.Unlock()
		loadModel = prev
	})
	path := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitReady(t *testing.T, a agentv1.Agent) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		st, err := a.Status(context.Background(), extkit.Internal)
		if err != nil {
			t.Fatal(err)
		}
		if st.Ready {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("not ready: %+v", st)
		}
	}
}

func extErr(err error) *extkit.Error {
	var e *extkit.Error
	errors.As(err, &e)
	return e
}

// No settings and no model: agent/v1 is there and says how to configure it.
func TestNotConfigured(t *testing.T) {
	withSection(t, `{}`)
	app := testApp(t)
	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	a, ok := agentv1.Contract.Of(app)
	if !ok {
		t.Fatal("agent/v1 not provided")
	}
	_, err := a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{Prompt: "hi"})
	if e := extErr(err); e == nil || e.Code != "not_configured" {
		t.Fatalf("complete: %v", err)
	}
	if _, err := a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{}); extErr(err) == nil || extErr(err).Code != "bad_request" {
		t.Fatalf("empty prompt: %v", err)
	}
}

// A model in the agent section: the runner answers, with the system prompt and JSON schema in the prompt, and the
// parsed JSON in data.
func TestRunnerAnswers(t *testing.T) {
	fake := &fakeModel{reply: "Sure:\n```json\n{\"label\": \"bug\"}\n```"}
	path := useFake(t, fake)
	withSection(t, `{"model": "`+path+`", "max_tokens": 50}`)
	app := testApp(t)
	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	serve(t, app)
	a := agentv1.Contract.Must(app)
	waitReady(t, a)
	res, err := a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{
		System: "Label it.", Prompt: "it crashes", Schema: json.RawMessage(`{"type":"object"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != "runner" || string(res.Data) != `{"label": "bug"}` || res.Usage.OutputTokens != 7 {
		t.Fatalf("result: %+v", res)
	}
	p, _ := fake.prompt.Load().(string)
	if !strings.Contains(p, "system: Label it.") || !strings.Contains(p, `JSON schema`) || !strings.Contains(p, "user: it crashes") {
		t.Fatalf("prompt: %q", p)
	}
}

// fakeAPI is an OpenAI-compatible chat completions endpoint.
func fakeAPI(t *testing.T, reply string) (*httptest.Server, *atomic.Value) {
	t.Helper()
	var got atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"error":{"message":"bad key"}}`)
			return
		}
		b, _ := io.ReadAll(r.Body)
		got.Store(string(b))
		if strings.Contains(string(b), `"stream":true`) {
			w.Header().Set("Content-Type", "text/event-stream")
			for _, piece := range strings.SplitAfter(reply, " ") {
				c, _ := json.Marshal(map[string]any{"model": "gpt-test", "choices": []any{map[string]any{"delta": map[string]any{"content": piece}}}})
				_, _ = io.WriteString(w, "data: "+string(c)+"\n\n")
			}
			_, _ = io.WriteString(w, `data: {"model":"gpt-test","choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
			_, _ = io.WriteString(w, `data: {"model":"gpt-test","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":3}}`+"\n\ndata: [DONE]\n\n")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   "gpt-test",
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": reply}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 11, "completion_tokens": 3},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

// The app's own API wins over the runner once its admins save it; the key is never read back, and an empty key keeps it.
func TestAppAPIWins(t *testing.T) {
	fake := &fakeModel{reply: "from the runner"}
	path := useFake(t, fake)
	withSection(t, `{"model": "`+path+`"}`)
	srv, got := fakeAPI(t, "from the API")
	app := testApp(t)
	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	var sec settingsv1.Section
	for _, s := range settingsv1.Sections(app) {
		if s.Info().Extension == Name {
			sec = s
		}
	}
	if sec == nil {
		t.Fatal("no settings section")
	}
	tester := sec.(settingsv1.Tester)
	if err := tester.Test(context.Background(), json.RawMessage(`{"mode": "api", "endpoint": "`+srv.URL+`/v1", "key": "wrong"}`)); err == nil || !strings.Contains(err.Error(), "bad key") {
		t.Fatalf("test with a bad key: %v", err)
	}
	saver := sec.(settingsv1.Saver)
	if _, err := saver.Save(context.Background(), json.RawMessage(`{"mode": "api", "endpoint": "`+srv.URL+`/v1", "key": "sk-test", "apiModel": "gpt-test"}`)); err != nil {
		t.Fatal(err)
	}
	view, _ := sec.Get(context.Background())
	if strings.Contains(string(view), "sk-test") || !strings.Contains(string(view), `"keySet":true`) {
		t.Fatalf("settings view leaks or loses the key: %s", view)
	}
	if _, err := saver.Save(context.Background(), json.RawMessage(`{"mode": "api", "endpoint": "`+srv.URL+`/v1", "key": "", "apiModel": "gpt-test"}`)); err != nil {
		t.Fatal(err)
	}
	a := agentv1.Contract.Must(app)
	res, err := a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{Prompt: "hi", MaxTokens: 9})
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != "app" || res.Text != "from the API" || res.Usage.InputTokens != 11 || fake.calls.Load() != 0 {
		t.Fatalf("result: %+v (runner calls %d)", res, fake.calls.Load())
	}
	body, _ := got.Load().(string)
	if !strings.Contains(body, `"max_tokens":9`) || !strings.Contains(body, `"model":"gpt-test"`) {
		t.Fatalf("request: %s", body)
	}
}

// The host's share: answers are capped at max_tokens, each instance has a few calls at once, and nothing is counted:
// an instance can call as often as it likes.
func TestHostShare(t *testing.T) {
	fake := &fakeModel{reply: "ok"}
	path := useFake(t, fake)
	cfg := Config{Model: path, PerInstance: PerInstance{Concurrency: 1, MaxTokens: 5}}
	r, err := sharedRunner(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-r.ready
	hs := newHostShare(r, cfg.withDefaults())
	a := &hostAgent{share: hs, id: "a"}
	b := &hostAgent{share: hs, id: "b"}
	in := agentv1.CompleteInput{Prompt: "hi", MaxTokens: 500}
	for i := 0; i < 20; i++ {
		if res, err := a.Complete(context.Background(), extkit.Internal, in); err != nil || res.Backend != "host" {
			t.Fatalf("call %d: %+v %v", i, res, err)
		}
	}
	if got := fake.lastMax.Load(); got != 5 {
		t.Fatalf("max_tokens cap: the model got %d, want 5", got)
	}
	if st, _ := a.Status(context.Background(), extkit.Internal); st.Backend != "host" || !st.Ready {
		t.Fatalf("status: %+v", st)
	}

	// one call at a time per instance: a second one while the first runs is refused at once; another instance isn't
	fake.delay = 300 * time.Millisecond
	done := make(chan error, 1)
	go func() { _, err := b.Complete(context.Background(), extkit.Internal, in); done <- err }()
	time.Sleep(50 * time.Millisecond)
	if _, err := b.Complete(context.Background(), extkit.Internal, in); extErr(err) == nil || extErr(err).Code != "busy" {
		t.Fatalf("second call at once: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestExtractJSON(t *testing.T) {
	for in, want := range map[string]string{
		`{"a":1}`:                         `{"a":1}`,
		"Here:\n```json\n[1,2]\n```\nbye": `[1,2]`,
		`The answer is {"a": {"b": 2}}.`:  `{"a": {"b": 2}}`,
	} {
		got, err := extractJSON(in)
		if err != nil || string(got) != want {
			t.Errorf("%q: %s %v", in, got, err)
		}
	}
	if _, err := extractJSON("no json here"); err == nil {
		t.Error("no JSON: want an error")
	}
}

func TestResolveModel(t *testing.T) {
	src, err := resolveModel("qwen3-1.7b", "/cache")
	if err != nil || src.Path != "/cache/qwen3-1.7b.gguf" || src.SHA256 == "" || !strings.HasPrefix(src.URL, "https://huggingface.co/Qwen/") {
		t.Fatalf("known: %+v %v", src, err)
	}
	if src, err := resolveModel("hf:org/repo/file.gguf", "/cache"); err != nil || src.SHA256 != "" || src.URL != "https://huggingface.co/org/repo/resolve/main/file.gguf" {
		t.Fatalf("hf: %+v %v", src, err)
	}
	if src, err := resolveModel("/models/x.gguf", "/cache"); err != nil || src.URL != "" || src.Path != "/models/x.gguf" {
		t.Fatalf("path: %+v %v", src, err)
	}
	if _, err := resolveModel("gpt-5", "/cache"); err == nil {
		t.Fatal("unknown name: want an error")
	}
}

// collect runs agent/v1's stream in this process, as $agent.stream does: the pieces, and the final result. stopAfter > 0
// stops it after that many pieces.
func collect(t *testing.T, a agentv1.Agent, in agentv1.CompleteInput, stopAfter int) ([]string, *agentv1.CompleteResult) {
	t.Helper()
	s, err := a.Stream(context.Background(), extkit.Internal, in)
	if err != nil {
		t.Fatal(err)
	}
	var pieces []string
	var res *agentv1.CompleteResult
	err = s.Run(context.Background(), func(e any) error {
		ch := e.(agentv1.Chunk)
		if ch.Done {
			res = ch.Result
			return nil
		}
		pieces = append(pieces, ch.Text)
		if stopAfter > 0 && len(pieces) >= stopAfter {
			return errStopped
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("no done chunk")
	}
	return pieces, res
}

// agent/v1's stream hands the answer over in pieces from the runner and from the app's own API, ends with the whole
// result, and stops when the receiver does.
func TestStream(t *testing.T) {
	fake := &fakeModel{reply: "one two three four"}
	path := useFake(t, fake)
	withSection(t, `{"model": "`+path+`"}`)
	app := testApp(t)
	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	serve(t, app)
	a := agentv1.Contract.Must(app)
	waitReady(t, a)

	pieces, res := collect(t, a, agentv1.CompleteInput{Prompt: "count"}, 0)
	if res.Text != "one two three four" || len(pieces) != 4 || res.Backend != "runner" {
		t.Fatalf("runner stream: %+v %q", res, pieces)
	}
	pieces, res = collect(t, a, agentv1.CompleteInput{Prompt: "count"}, 2)
	if res.StopReason != "cancelled" || res.Text != "one two " || len(pieces) != 2 {
		t.Fatalf("stopped stream: %+v %q", res, pieces)
	}
	if _, err := a.Stream(context.Background(), extkit.Internal, agentv1.CompleteInput{}); extErr(err) == nil || extErr(err).Code != "bad_request" {
		t.Fatalf("empty prompt: %v", err)
	}

	srv, got := fakeAPI(t, "from the API stream")
	if err := SaveSettings(app, Settings{Mode: ModeAPI, Endpoint: srv.URL + "/v1", Key: "sk-test"}); err != nil {
		t.Fatal(err)
	}
	pieces, res = collect(t, a, agentv1.CompleteInput{Prompt: "hi"}, 0)
	if res.Text != "from the API stream" || len(pieces) != 4 || res.Usage.OutputTokens != 3 || res.StopReason != "stop" || res.Backend != "app" {
		t.Fatalf("API stream: %+v %q", res, pieces)
	}
	if body, _ := got.Load().(string); !strings.Contains(body, `"stream":true`) || !strings.Contains(body, `"include_usage":true`) {
		t.Fatalf("request: %s", body)
	}
}

func section(t *testing.T, app core.App) settingsv1.Section {
	t.Helper()
	for _, s := range settingsv1.Sections(app) {
		if s.Info().Extension == Name {
			return s
		}
	}
	t.Fatal("no LLM settings section")
	return nil
}

func view(t *testing.T, sec settingsv1.Section) settingsView {
	t.Helper()
	raw, err := sec.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var v settingsView
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// Settings → LLM: pocketone.yaml's model runs until the admins choose; off stops answers; another built-in model
// replaces the running one (which is unloaded); mode "" goes back to pocketone.yaml; a new endpoint drops the saved key.
func TestSettingsModes(t *testing.T) {
	fake := &fakeModel{reply: "ok"}
	path := useFake(t, fake)
	other := filepath.Join(filepath.Dir(path), "other.gguf")
	if err := os.WriteFile(other, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	withSection(t, `{"model": "`+path+`"}`)
	app := testApp(t)
	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	serve(t, app)
	a := agentv1.Contract.Must(app)
	sec := section(t, app)
	saver := sec.(settingsv1.Saver)
	save := func(v string) error { _, err := saver.Save(context.Background(), json.RawMessage(v)); return err }

	if v := view(t, sec); v.Effective != ModeBuiltin || v.Source != "pocketone.yaml" || v.EffectiveModel != path || len(v.Models) != len(knownModels)+1 {
		t.Fatalf("before any choice: %+v", v)
	}
	waitReady(t, a)
	if _, err := a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{Prompt: "hi"}); err != nil {
		t.Fatal(err)
	}

	if err := save(`{"mode": "off"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{Prompt: "hi"}); extErr(err) == nil || !strings.Contains(err.Error(), "Settings → LLM") {
		t.Fatalf("off: %v", err)
	}
	if st, _ := a.Status(context.Background(), extkit.Internal); st.Backend != "" || st.Ready {
		t.Fatalf("off status: %+v", st)
	}
	runnersMu.Lock()
	n := len(runners)
	runnersMu.Unlock()
	if n != 0 {
		t.Fatalf("off: %d runners still loaded", n)
	}

	if err := save(`{"mode": "builtin", "model": "gpt-5"}`); extErr(err) == nil || extErr(err).Code != "bad_request" {
		t.Fatalf("unknown model: %v", err)
	}
	if err := save(`{"mode": "builtin", "model": "` + path + `"}`); err != nil { // pocketone.yaml's own model is a choice too
		t.Fatal(err)
	}
	if v := view(t, sec); v.Effective != ModeBuiltin || v.Source != "settings" {
		t.Fatalf("builtin chosen: %+v", v)
	}

	if err := save(`{"mode": ""}`); err != nil {
		t.Fatal(err)
	}
	if v := view(t, sec); v.Source != "pocketone.yaml" || v.Mode != "" {
		t.Fatalf("back to pocketone.yaml: %+v", v)
	}

	if err := save(`{"mode": "api", "endpoint": "https://one.example/v1", "key": "k1"}`); err != nil {
		t.Fatal(err)
	}
	if err := save(`{"mode": "api", "endpoint": "https://two.example/v1", "key": ""}`); err != nil {
		t.Fatal(err)
	}
	if cur, _ := LoadSettings(app); cur.Key != "" {
		t.Fatalf("the key followed a new endpoint: %+v", cur)
	}
	if err := save(`{"mode": "host"}`); extErr(err) == nil {
		t.Fatal("host mode on a single app: want an error")
	}
	_ = other
}

// Under a host, the section offers off, the host's model or an API; built in is the host's call.
func TestHostedSettings(t *testing.T) {
	withSection(t, `{"model": "qwen3-1.7b"}`)
	app := testApp(t)
	sec := settingsSection{app: app, hosted: true}
	if _, err := sec.Save(context.Background(), json.RawMessage(`{"mode": "builtin", "model": "qwen3-0.6b"}`)); extErr(err) == nil {
		t.Fatal("builtin under a host: want an error")
	}
	v := view(t, sec)
	if !v.Hosted || v.Effective != ModeHost || v.Source != "host" || v.EffectiveModel != "qwen3-1.7b" || len(v.Models) != 0 {
		t.Fatalf("hosted view: %+v", v)
	}
	if _, err := sec.Save(context.Background(), json.RawMessage(`{"mode": "off"}`)); err != nil {
		t.Fatal(err)
	}
	if c := choose(app, Config{}, true); c.Mode != ModeOff {
		t.Fatalf("hosted off: %+v", c)
	}
}
