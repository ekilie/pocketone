package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Warming returns the same id for the same text, pins it in the model's cache (with wait, once it's done), and Forget
// releases it.
func TestWarmAndForget(t *testing.T) {
	fake := &fakeModel{reply: "ok"}
	path := useFake(t, fake)
	withSection(t, `{"model": "`+path+`"}`)
	app := testApp(t)
	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	serve(t, app)
	a := agentv1.Contract.Must(app)
	waitReady(t, a)
	in := agentv1.WarmInput{System: "You help with Acme Notes.", Wait: "5s"}
	w, err := a.Warm(context.Background(), extkit.Internal, in)
	if err != nil || !w.Warmed || w.ID == "" || w.Tokens == 0 {
		t.Fatalf("warm: %+v %v", w, err)
	}
	again, _ := a.Warm(context.Background(), extkit.Internal, agentv1.WarmInput{System: "You help with Acme Notes."})
	if again.ID != w.ID {
		t.Fatalf("same text, another id: %s vs %s", again.ID, w.ID)
	}
	if other, _ := a.Warm(context.Background(), extkit.Internal, agentv1.WarmInput{System: "Something else."}); other.ID == w.ID {
		t.Fatal("other text, same id")
	}
	waitFor(t, "two pins", func() bool { return fake.pinned() == 2 })
	if _, err := a.Forget(context.Background(), extkit.Internal, agentv1.ForgetInput{ID: w.ID}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "one pin left", func() bool { return fake.pinned() == 1 })
}

// A call with a thread passes it to the model with the app's folder in pb_cache; Threads lists it and ForgetThread
// drops it. Count is exact with the built-in model.
func TestThreadsAndCount(t *testing.T) {
	fake := &fakeModel{reply: "ok"}
	path := useFake(t, fake)
	withSection(t, `{"model": "`+path+`"}`)
	app := testApp(t)
	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	serve(t, app)
	a := agentv1.Contract.Must(app)
	waitReady(t, a)
	if _, err := a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{Prompt: "hi", Thread: "chat-1"}); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	dir := fake.dirs["chat-1"]
	fake.mu.Unlock()
	if want := filepath.Join(filepath.Dir(app.DataDir()), "pb_cache", "agent", "threads"); dir != want {
		t.Fatalf("thread folder %q, want %q", dir, want)
	}
	list, _ := a.Threads(context.Background(), extkit.Internal)
	if len(list.Threads) != 1 || list.Threads[0].ID != "chat-1" {
		t.Fatalf("threads: %+v", list)
	}
	_, _ = a.ForgetThread(context.Background(), extkit.Internal, agentv1.ForgetInput{ID: "chat-1"})
	if list, _ := a.Threads(context.Background(), extkit.Internal); len(list.Threads) != 0 {
		t.Fatalf("after forget: %+v", list)
	}
	c, _ := a.Count(context.Background(), extkit.Internal, agentv1.CountInput{Text: "one two three"})
	if !c.Exact || c.Tokens != 3 {
		t.Fatalf("count: %+v", c)
	}
}

// While the model loads, a call fails at once with not_ready, unless it says how long to wait.
func TestWaitForModel(t *testing.T) {
	fake := &fakeModel{reply: "ok"}
	path := useFake(t, fake)
	release := make(chan struct{})
	loadModel = func(string, Config) (localModel, error) { <-release; return fake, nil }
	withSection(t, `{"model": "`+path+`"}`)
	app := testApp(t)
	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	serve(t, app)
	a := agentv1.Contract.Must(app)
	if _, err := a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{Prompt: "hi"}); extErr(err) == nil || extErr(err).Code != "not_ready" {
		t.Fatalf("without wait: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{Prompt: "hi", Wait: "10s"})
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("with wait: %v", err)
	}
}

// On a host, an instance's warms count against its pinned memory, its threads are its own, and its warms go when it
// stops.
func TestHostWarmsAndThreads(t *testing.T) {
	fake := &fakeModel{reply: "ok"}
	path := useFake(t, fake)
	cfg := Config{Model: path, PerInstance: PerInstance{PinnedMB: 1}}
	r, err := sharedRunner(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-r.ready
	hs := newHostShare(r, cfg.withDefaults())
	a := &hostAgent{share: hs, id: "a"}
	b := &hostAgent{share: hs, id: "b"}
	big := strings.Repeat("word ", 8000) // the fake keeps len/4 tokens at 1000 bytes each: well over 1 MB
	if w, _ := a.Warm(context.Background(), extkit.Internal, agentv1.WarmInput{System: big, Wait: "5s"}); !w.Warmed {
		t.Fatalf("first warm: %+v", w)
	}
	if w, _ := a.Warm(context.Background(), extkit.Internal, agentv1.WarmInput{System: "more", Wait: "5s"}); w.Warmed {
		t.Fatalf("over the instance's pinned memory: %+v", w)
	}
	if w, _ := b.Warm(context.Background(), extkit.Internal, agentv1.WarmInput{System: "b's own", Wait: "5s"}); !w.Warmed {
		t.Fatalf("another instance: %+v", w)
	}
	_, _ = a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{Prompt: "hi", Thread: "t"})
	if l, _ := b.Threads(context.Background(), extkit.Internal); len(l.Threads) != 0 {
		t.Fatalf("b sees a's threads: %+v", l)
	}
	if l, _ := a.Threads(context.Background(), extkit.Internal); len(l.Threads) != 1 {
		t.Fatalf("a's threads: %+v", l)
	}
	hs.releaseInstance("a")
	waitFor(t, "a's pin released", func() bool { return fake.pinned() == 1 })
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("gave up waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
