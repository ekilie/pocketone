package agentchat_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	"github.com/pockethost/pocketone/ext/agent"
	agentchat "github.com/pockethost/pocketone/ext/agent-chat"
	chatv1 "github.com/pockethost/pocketone/ext/contracts/agent/chat/v1"
	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/js"
	"github.com/pockethost/pocketone/ext/jsvm"
)

// fakeAgent answers "answer N" (or "SUMMARY" when asked to summarize) and remembers every input.
type fakeAgent struct {
	mu        sync.Mutex
	inputs    []agentv1.CompleteInput
	forgotten []string
}

func (f *fakeAgent) answer(in agentv1.CompleteInput) agentv1.CompleteResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inputs = append(f.inputs, in)
	if strings.HasPrefix(in.System, "Update the summary") {
		return agentv1.CompleteResult{Text: "SUMMARY", Backend: "runner"}
	}
	return agentv1.CompleteResult{Text: "answer " + string(rune('0'+len(f.inputs))), Backend: "runner"}
}

// last is the last answer's input (summaries left out).
func (f *fakeAgent) last() agentv1.CompleteInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.inputs) - 1; i >= 0; i-- {
		if !strings.HasPrefix(f.inputs[i].System, "Update the summary") {
			return f.inputs[i]
		}
	}
	return agentv1.CompleteInput{}
}

func (f *fakeAgent) Complete(_ context.Context, _ extkit.Auth, in agentv1.CompleteInput) (agentv1.CompleteResult, error) {
	return f.answer(in), nil
}
func (f *fakeAgent) Stream(_ context.Context, _ extkit.Auth, in agentv1.CompleteInput) (extkit.Stream, error) {
	return extkit.Stream{Run: func(ctx context.Context, send func(any) error) error {
		res := f.answer(in)
		for _, w := range strings.SplitAfter(res.Text, " ") {
			if err := send(agentv1.Chunk{Text: w}); err != nil {
				return err
			}
		}
		return send(agentv1.Chunk{Done: true, Result: &res})
	}}, nil
}
func (f *fakeAgent) Status(context.Context, extkit.Auth) (agentv1.Status, error) {
	return agentv1.Status{Backend: "runner", Ready: true}, nil
}
func (f *fakeAgent) Warm(context.Context, extkit.Auth, agentv1.WarmInput) (agentv1.WarmResult, error) {
	return agentv1.WarmResult{}, nil
}
func (f *fakeAgent) Forget(context.Context, extkit.Auth, agentv1.ForgetInput) (agentv1.Done, error) {
	return agentv1.Done{OK: true}, nil
}
func (f *fakeAgent) Count(context.Context, extkit.Auth, agentv1.CountInput) (agentv1.CountResult, error) {
	return agentv1.CountResult{}, nil
}
func (f *fakeAgent) Threads(context.Context, extkit.Auth) (agentv1.ThreadList, error) {
	return agentv1.ThreadList{}, nil
}
func (f *fakeAgent) ForgetThread(_ context.Context, _ extkit.Auth, in agentv1.ForgetInput) (agentv1.Done, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forgotten = append(f.forgotten, in.ID)
	return agentv1.Done{OK: true}, nil
}

// fakeExt provides the fake as agent/v1.
type fakeExt struct{ a *fakeAgent }

func (fakeExt) Name() string { return "fake-agent" }
func (fakeExt) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("agent/v1")}
}
func (e fakeExt) Register(app core.App, _ extkit.Config) error {
	agentv1.Contract.Provide(app, e.a)
	return nil
}

func setup(t *testing.T) (core.App, *fakeAgent, chatv1.Chats) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	fa := &fakeAgent{}
	if _, err := extkit.Setup(app, []extkit.Extension{fakeExt{fa}, agentchat.New()}, nil, nil); err != nil {
		t.Fatal(err)
	}
	return app, fa, chatv1.Contract.Must(app)
}

// A chat keeps its messages and sends them back each turn on its thread; once enough pile up past the window, the
// oldest are folded into the summary, which leads the next prompt.
func TestSendAndFold(t *testing.T) {
	_, fa, c := setup(t)
	ctx := context.Background()
	chat, err := c.New(ctx, extkit.Internal, chatv1.NewInput{Title: "Support", Meta: json.RawMessage(`{"user":"u1"}`)})
	if err != nil || chat.ID == "" || chat.Title != "Support" {
		t.Fatalf("new: %+v %v", chat, err)
	}
	send := func(text string) chatv1.SendResult {
		t.Helper()
		r, err := c.Send(ctx, extkit.Internal, chatv1.SendInput{ID: chat.ID, Text: text, System: "Be brief.", Window: 2,
			SummarizeEvery: 2, Context: "CTX for " + text})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	r1 := send("q1")
	in := fa.last()
	if in.Thread != chat.ID || in.System != "Be brief." || len(in.Messages) != 1 || in.Messages[0].Content != "q1\n\nCTX for q1" {
		t.Fatalf("first turn's input: %+v", in)
	}
	if r1.Answer.Text == "" || r1.Summarized {
		t.Fatalf("first answer: %+v", r1)
	}
	send("q2") // 4 messages: 2 past the window of 2, which is SummarizeEvery: fold
	if got, _ := c.Get(ctx, extkit.Internal, chatv1.Ref{ID: chat.ID}); got.Summary != "SUMMARY" || got.Messages != 4 {
		t.Fatalf("after the fold: %+v", got)
	}
	send("q3")
	in = fa.last()
	if len(in.Messages) != 4 || !strings.Contains(in.Messages[0].Content, "SUMMARY") || in.Messages[1].Content != "q2" ||
		in.Messages[3].Content != "q3\n\nCTX for q3" {
		t.Fatalf("after the fold, the prompt is the summary, the turns since, and the question: %+v", in.Messages)
	}
	// history keeps every turn (the question without its context), oldest first, and pages with before
	h, _ := c.History(ctx, extkit.Internal, chatv1.HistoryInput{ID: chat.ID})
	if len(h.Messages) != 6 || h.Messages[0].Content != "q1" || h.Messages[0].Role != "user" || h.Messages[1].Role != "assistant" {
		t.Fatalf("history: %+v", h.Messages)
	}
	page, _ := c.History(ctx, extkit.Internal, chatv1.HistoryInput{ID: chat.ID, Limit: 2, Before: h.Messages[4].ID})
	if len(page.Messages) != 2 || page.Messages[0].Content != "q2" {
		t.Fatalf("page: %+v", page.Messages)
	}
}

// Streaming sends the agent's chunks and saves both turns before the last one.
func TestSendStream(t *testing.T) {
	_, _, c := setup(t)
	ctx := context.Background()
	chat, _ := c.New(ctx, extkit.Internal, chatv1.NewInput{})
	s, err := c.SendStream(ctx, extkit.Internal, chatv1.SendInput{ID: chat.ID, Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	var pieces []string
	var done bool
	err = s.Run(ctx, func(e any) error {
		ch := e.(agentv1.Chunk)
		if ch.Done {
			done = true
			if got, _ := c.Get(ctx, extkit.Internal, chatv1.Ref{ID: chat.ID}); got.Messages != 2 {
				t.Errorf("not saved before the last chunk: %+v", got)
			}
			return nil
		}
		pieces = append(pieces, ch.Text)
		return nil
	})
	if err != nil || !done || strings.Join(pieces, "") != "answer 1" {
		t.Fatalf("stream: %v %v %q", err, done, pieces)
	}
}

// Chats are found by their meta, updated, and forgotten one at a time or by meta (the model's state with them).
func TestListUpdateForget(t *testing.T) {
	_, fa, c := setup(t)
	ctx := context.Background()
	a, _ := c.New(ctx, extkit.Internal, chatv1.NewInput{Meta: json.RawMessage(`{"user":"u1","kind":"support"}`)})
	b, _ := c.New(ctx, extkit.Internal, chatv1.NewInput{Meta: json.RawMessage(`{"user":"u1"}`)})
	_, _ = c.New(ctx, extkit.Internal, chatv1.NewInput{Meta: json.RawMessage(`{"user":"u2"}`)})
	if l, _ := c.List(ctx, extkit.Internal, chatv1.ListInput{Meta: map[string]any{"user": "u1"}}); len(l.Chats) != 2 {
		t.Fatalf("u1's chats: %+v", l)
	}
	if l, _ := c.List(ctx, extkit.Internal, chatv1.ListInput{Meta: map[string]any{"user": "u1", "kind": "support"}}); len(l.Chats) != 1 || l.Chats[0].ID != a.ID {
		t.Fatalf("u1's support chats: %+v", l)
	}
	if _, err := c.List(ctx, extkit.Internal, chatv1.ListInput{Meta: map[string]any{"x) || 1=1 || (": "y"}}); err == nil {
		t.Fatal("a bad meta key was accepted")
	}
	title := "Renamed"
	if u, err := c.Update(ctx, extkit.Internal, chatv1.UpdateInput{ID: b.ID, Title: &title}); err != nil || u.Title != "Renamed" || !strings.Contains(string(u.Meta), "u1") {
		t.Fatalf("update: %+v %v", u, err)
	}
	if f, err := c.ForgetWhere(ctx, extkit.Internal, chatv1.ListInput{Meta: map[string]any{"user": "u1"}}); err != nil || f.Count != 2 {
		t.Fatalf("forget u1: %+v %v", f, err)
	}
	if l, _ := c.List(ctx, extkit.Internal, chatv1.ListInput{}); len(l.Chats) != 1 {
		t.Fatalf("left: %+v", l)
	}
	if _, err := c.Get(ctx, extkit.Internal, chatv1.Ref{ID: a.ID}); err == nil {
		t.Fatal("a forgotten chat is still there")
	}
	if len(fa.forgotten) != 2 {
		t.Fatalf("threads forgotten: %v", fa.forgotten)
	}
}

const hooks = `onRecordCreate((e) => {
  const id = $agent.newChat({ title: "From JS", meta: { user: "u9" } })
  const chat = $agent.chat(id, { system: "Be brief.", context: (q) => "ctx:" + q })
  const a1 = chat.send("hello")
  const pieces = []
  const a2 = chat.send("again", { onText: (t) => { pieces.push(t) } })
  chat.title("Renamed")
  const out = {
    a1: a1.text, a2: a2.text, pieces: pieces.join("|"), history: chat.history().map((m) => m.role + ":" + m.content),
    title: chat.title(), meta: chat.meta(), listed: $agent.chats.list({ meta: { user: "u9" } }).length,
  }
  out.forgotten = $agent.chats.forget({ meta: { user: "u9" } })
  e.record.set("result", JSON.stringify(out))
  e.next()
}, "probes")

onRecordCreate(async (e) => {
  const chat = $agent.chat($agent.newChat())
  const a = await chat.send.async("async hello")
  e.record.set("result", a.text)
  e.next()
}, "asyncprobes")
`

// $agent's chat methods from pb_hooks, over agent-chat (bound by ext/agent's BindJS whoever provides agent/v1).
func TestJS(t *testing.T) {
	app, fa, _ := setup(t)
	for _, name := range []string{"probes", "asyncprobes"} {
		c := core.NewBaseCollection(name)
		c.Fields.Add(&core.TextField{Name: "result", Max: 10000})
		if err := app.Save(c); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(t.TempDir(), "pb_hooks")
	_ = os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, "main.pb.js"), []byte(hooks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := jsvm.Register(app, jsvm.Config{HooksDir: dir, OnInit: func(rt js.Runtime) { agent.New().BindJS(app, rt) }}); err != nil {
		t.Fatal(err)
	}
	if err := app.OnBootstrap().Trigger(&core.BootstrapEvent{App: app}, func(*core.BootstrapEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}

	r := core.NewRecord(mustCollection(t, app, "probes"))
	if err := app.Save(r); err != nil {
		t.Fatal(err)
	}
	var out struct {
		A1, A2, Pieces, Title string
		History               []string
		Meta                  map[string]any
		Listed, Forgotten     int
	}
	if err := json.Unmarshal([]byte(r.GetString("result")), &out); err != nil {
		t.Fatalf("result %q: %v", r.GetString("result"), err)
	}
	if out.A1 != "answer 1" || out.A2 != "answer 2" || out.Pieces != "answer |2" || out.Title != "Renamed" ||
		strings.Join(out.History, ",") != "user:hello,assistant:answer 1,user:again,assistant:answer 2" ||
		out.Meta["user"] != "u9" || out.Listed != 1 || out.Forgotten != 1 {
		t.Fatalf("from JS: %+v", out)
	}
	if q := fa.inputs[0].Messages[len(fa.inputs[0].Messages)-1].Content; q != "hello\n\nctx:hello" {
		t.Fatalf("context function: %q", q)
	}

	ar := core.NewRecord(mustCollection(t, app, "asyncprobes"))
	if err := app.Save(ar); err != nil {
		t.Fatal(err)
	}
	if got := ar.GetString("result"); got != "answer 3" {
		t.Fatalf("send.async: %q", got)
	}
}

func mustCollection(t *testing.T, app core.App, name string) *core.Collection {
	t.Helper()
	c, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
