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

	streamv1 "github.com/pockethost/pocketone/ext/contracts/agent/stream/v1"
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
	reply  string
	calls  atomic.Int32
	prompt atomic.Value
	delay  time.Duration
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
	if err := tester.Test(context.Background(), json.RawMessage(`{"endpoint": "`+srv.URL+`/v1", "key": "wrong"}`)); err == nil || !strings.Contains(err.Error(), "bad key") {
		t.Fatalf("test with a bad key: %v", err)
	}
	saver := sec.(settingsv1.Saver)
	if _, err := saver.Save(context.Background(), json.RawMessage(`{"endpoint": "`+srv.URL+`/v1", "key": "sk-test", "model": "gpt-test"}`)); err != nil {
		t.Fatal(err)
	}
	view, _ := sec.Get(context.Background())
	if strings.Contains(string(view), "sk-test") || !strings.Contains(string(view), `"keySet":true`) {
		t.Fatalf("settings view leaks or loses the key: %s", view)
	}
	if _, err := saver.Save(context.Background(), json.RawMessage(`{"endpoint": "`+srv.URL+`/v1", "key": "", "model": "gpt-test"}`)); err != nil {
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

// The host's share: each instance has a few calls at once and a daily allowance, which rolls over at midnight UTC.
func TestHostShare(t *testing.T) {
	fake := &fakeModel{reply: "ok"}
	path := useFake(t, fake)
	cfg := Config{Model: path, PerInstance: PerInstance{TokensPerDay: 10, Concurrency: 1, MaxTokens: 5}}
	r, err := sharedRunner(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-r.ready
	hs := newHostShare(r, cfg.withDefaults())
	day := time.Date(2026, 11, 18, 23, 0, 0, 0, time.UTC)
	hs.now = func() time.Time { return day }
	a := &hostAgent{share: hs, id: "a"}
	b := &hostAgent{share: hs, id: "b"}
	in := agentv1.CompleteInput{Prompt: "hi", MaxTokens: 500}
	for i := 0; i < 2; i++ { // 7 + 7 tokens: the second call is allowed (under 10 before it), the third isn't
		if res, err := a.Complete(context.Background(), extkit.Internal, in); err != nil || res.Backend != "host" {
			t.Fatalf("call %d: %+v %v", i, res, err)
		}
	}
	if _, err := a.Complete(context.Background(), extkit.Internal, in); extErr(err) == nil || extErr(err).Code != "quota" {
		t.Fatalf("over the allowance: %v", err)
	}
	if _, err := b.Complete(context.Background(), extkit.Internal, in); err != nil {
		t.Fatalf("another instance has its own allowance: %v", err)
	}
	st, _ := a.Status(context.Background(), extkit.Internal)
	if st.Quota == nil || st.Quota.UsedToday != 14 || st.Quota.MaxTokens != 5 {
		t.Fatalf("status: %+v", st.Quota)
	}
	day = day.Add(2 * time.Hour)
	if _, err := a.Complete(context.Background(), extkit.Internal, in); err != nil {
		t.Fatalf("a new day: %v", err)
	}

	// one call at a time per instance: a second one while the first runs is refused at once
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

// agent.stream/v1 hands the answer over in pieces from the runner and from the app's own API, and a callback returning
// false stops it.
func TestStream(t *testing.T) {
	fake := &fakeModel{reply: "one two three four"}
	path := useFake(t, fake)
	withSection(t, `{"model": "`+path+`"}`)
	app := testApp(t)
	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	serve(t, app)
	waitReady(t, agentv1.Contract.Must(app))
	s := streamv1.Contract.Must(app)

	var pieces []string
	res, err := s.Stream(context.Background(), streamv1.Input{CompleteInput: agentv1.CompleteInput{Prompt: "count"},
		OnText: func(p string) bool { pieces = append(pieces, p); return true }})
	if err != nil || res.Text != "one two three four" || len(pieces) != 4 || res.Backend != "runner" {
		t.Fatalf("runner stream: %+v %q %v", res, pieces, err)
	}
	pieces = nil
	res, _ = s.Stream(context.Background(), streamv1.Input{CompleteInput: agentv1.CompleteInput{Prompt: "count"},
		OnText: func(p string) bool { pieces = append(pieces, p); return len(pieces) < 2 }})
	if res.StopReason != "cancelled" || res.Text != "one two " || len(pieces) != 2 {
		t.Fatalf("stopped stream: %+v %q", res, pieces)
	}

	srv, got := fakeAPI(t, "from the API stream")
	if err := SaveSettings(app, Settings{Endpoint: srv.URL + "/v1", Key: "sk-test"}); err != nil {
		t.Fatal(err)
	}
	pieces = nil
	res, err = s.Stream(context.Background(), streamv1.Input{CompleteInput: agentv1.CompleteInput{Prompt: "hi"},
		OnText: func(p string) bool { pieces = append(pieces, p); return true }})
	if err != nil || res.Text != "from the API stream" || len(pieces) != 4 || res.Usage.OutputTokens != 3 || res.StopReason != "stop" {
		t.Fatalf("API stream: %+v %q %v", res, pieces, err)
	}
	if body, _ := got.Load().(string); !strings.Contains(body, `"stream":true`) || !strings.Contains(body, `"include_usage":true`) {
		t.Fatalf("request: %s", body)
	}
}
