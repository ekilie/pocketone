package search

import (
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"

	llmc "pocketone.dev/ext/contracts/llm"
	embedv1 "pocketone.dev/ext/contracts/llm/embed/v1"
	searchv1 "pocketone.dev/ext/contracts/search/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/js"
	"pocketone.dev/ext/jsvm"
)

func init() {
	background = false
	RequestInterval = 0
	retryWait = 0
}

// fakeEmbed makes a bag-of-words vector (each word hashed to one of 64 dimensions), so texts that share words are
// close; it remembers every input and fails while fail is set.
type fakeEmbed struct {
	mu     sync.Mutex
	inputs []string
	calls  int
	fail   error
}

func (f *fakeEmbed) Embed(_ context.Context, _ extkit.Auth, in embedv1.EmbedInput) (embedv1.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.fail != nil {
		return embedv1.Result{}, f.fail
	}
	res := embedv1.Result{Model: in.Model, Usage: llmc.Usage{CostMicroUSD: int64(len(in.Input))}}
	dims := 64
	if in.Dimensions > 0 {
		dims = in.Dimensions
	}
	for _, text := range in.Input {
		f.inputs = append(f.inputs, text)
		v := make([]float64, dims)
		for _, w := range wordRe.FindAllString(strings.ToLower(text), -1) {
			h := fnv.New32a()
			_, _ = h.Write([]byte(w))
			v[h.Sum32()%uint32(dims)]++
		}
		res.Vectors = append(res.Vectors, v)
	}
	return res, nil
}

func (f *fakeEmbed) Models(context.Context, extkit.Auth, llmc.ModelsInput) (llmc.Models, error) {
	return llmc.Models{}, nil
}

func (f *fakeEmbed) embedded() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.inputs)
}

type fakeExt struct{ e *fakeEmbed }

func (fakeExt) Name() string { return "fake-embed" }
func (fakeExt) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("llm.embed/v1")}
}
func (x fakeExt) Register(app core.App, _ extkit.Config) error {
	embedv1.Contract.Provide(app, x.e)
	return nil
}

func setup(t *testing.T) (core.App, *fakeEmbed, searchv1.Search) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	fe := &fakeEmbed{}
	if _, err := extkit.Setup(app, []extkit.Extension{fakeExt{fe}, New()}, nil, nil); err != nil {
		t.Fatal(err)
	}
	return app, fe, searchv1.Contract.Must(app)
}

var ctx = context.Background()

const guide = `# Lighthouse

The keeper climbs the stairs every night to light the lamp.

## The fox

A red fox lives in the dunes and visits the lighthouse at dawn.

## Boats

Fishing boats follow the beam home through the fog.
`

func drain(t *testing.T, app core.App) {
	t.Helper()
	if err := of(app).drain(ctx); err != nil {
		t.Fatal(err)
	}
}

func mustSearch(t *testing.T, s searchv1.Search, in searchv1.SearchInput) searchv1.Results {
	t.Helper()
	r, err := s.Search(ctx, extkit.Internal, in)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// An app ensures its bucket; ensuring again keeps its settings and fills only what was empty.
func TestEnsure(t *testing.T) {
	_, _, s := setup(t)
	b, err := s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "storytime", Model: "p/embed", Owner: "Story Time"})
	if err != nil || b.Model != "p/embed" || b.MaxDocuments != DefaultMaxDocuments || b.Owner != "Story Time" {
		t.Fatalf("ensure: %+v %v", b, err)
	}
	b, err = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "storytime", Title: "Library", Model: "other/model", Owner: "x"})
	if err != nil || b.Title != "Library" || b.Model != "p/embed" || b.Owner != "Story Time" {
		t.Fatalf("ensure again: %+v %v", b, err)
	}
	if _, err := s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "../x"}); !errors.Is(err, searchv1.ErrInvalidName) {
		t.Fatalf("bad name: %v", err)
	}
	if l, _ := s.Buckets(ctx, extkit.Internal); len(l.Buckets) != 1 {
		t.Fatalf("buckets: %+v", l)
	}
}

// A document is split at once and embedded in the background; search finds it by meaning and by keywords.
func TestAddEmbedSearch(t *testing.T) {
	app, fe, s := setup(t)
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "kb", Model: "p/embed"})
	d, err := s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "stories/lighthouse.md", Text: guide})
	if err != nil || d.Status != StatusQueued || d.Chunks != 3 || d.Kind != KindMarkdown {
		t.Fatalf("add: %+v %v", d, err)
	}
	drain(t, app)
	docs, _ := s.Documents(ctx, extkit.Internal, searchv1.DocumentsInput{Bucket: "kb"})
	if len(docs.Documents) != 1 || docs.Documents[0].Status != StatusReady || docs.Documents[0].CostMicroUSD != 3 {
		t.Fatalf("after embedding: %+v", docs)
	}
	if fe.embedded() != 3 || !strings.HasPrefix(fe.inputs[1], "stories/lighthouse.md — Lighthouse › The fox\n\n") {
		t.Fatalf("embedded: %q", fe.inputs)
	}

	for _, mode := range []string{"", searchv1.ModeSemantic, searchv1.ModeKeyword} {
		r := mustSearch(t, s, searchv1.SearchInput{Bucket: "kb", Query: "red fox dunes", Mode: mode, Limit: 2})
		if len(r.Results) == 0 || r.Results[0].Heading != "Lighthouse › The fox" || r.Results[0].Document != "stories/lighthouse.md" {
			t.Fatalf("mode %q: %+v", mode, r)
		}
		if mode == "" && (r.Mode != searchv1.ModeHybrid || r.Results[0].Semantic <= 0 || r.Results[0].Keyword <= 0) {
			t.Fatalf("hybrid: %+v", r)
		}
	}
	if r := mustSearch(t, s, searchv1.SearchInput{Bucket: "kb", Query: "fog", Prefix: "other/"}); len(r.Results) != 0 {
		t.Fatalf("prefix: %+v", r)
	}
	// FTS operators in a query are words, not syntax
	if _, err := s.Search(ctx, extkit.Internal, searchv1.SearchInput{Bucket: "kb", Query: `fox" OR NEAR(*`, Mode: "keyword"}); err != nil {
		t.Fatal(err)
	}
	b, _ := s.Bucket(ctx, extkit.Internal, searchv1.Ref{Name: "kb"})
	if b.Documents != 1 || b.Chunks != 3 || b.Pending != 0 || b.Bytes != int64(len(guide)) {
		t.Fatalf("bucket: %+v", b)
	}
}

// The same text again costs nothing; a changed document embeds only its new chunks.
func TestReingest(t *testing.T) {
	app, fe, s := setup(t)
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "kb", Model: "p/embed"})
	_, _ = s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "a.md", Text: guide})
	drain(t, app)
	d, err := s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "a.md", Text: guide})
	if err != nil || !d.Unchanged || d.Status != StatusReady {
		t.Fatalf("again: %+v %v", d, err)
	}
	changed := strings.Replace(guide, "follow the beam", "trust the beam", 1)
	d, _ = s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "a.md", Text: changed})
	if d.Status != StatusQueued {
		t.Fatalf("changed: %+v", d)
	}
	drain(t, app)
	if fe.embedded() != 4 || !strings.Contains(fe.inputs[3], "trust the beam") {
		t.Fatalf("re-embedded more than the changed chunk: %q", fe.inputs)
	}
	if r := mustSearch(t, s, searchv1.SearchInput{Bucket: "kb", Query: "trust", Mode: "keyword"}); len(r.Results) != 1 {
		t.Fatalf("new text not indexed: %+v", r)
	}
	if r := mustSearch(t, s, searchv1.SearchInput{Bucket: "kb", Query: "follow", Mode: "keyword"}); len(r.Results) != 0 {
		t.Fatalf("old text still indexed: %+v", r)
	}

	// a new model embeds everything again
	model := "p/other"
	if _, err := s.Update(ctx, extkit.Internal, searchv1.UpdateInput{Name: "kb", Model: &model}); err != nil {
		t.Fatal(err)
	}
	drain(t, app)
	if fe.embedded() != 7 {
		t.Fatalf("after a model change: %d embedded", fe.embedded())
	}
	// reindex splits again from the kept text
	if n, err := s.Reindex(ctx, extkit.Internal, searchv1.RemoveInput{Bucket: "kb"}); err != nil || n.Count != 1 {
		t.Fatalf("reindex: %+v %v", n, err)
	}
}

// Without a model a bucket is searched by keywords, and nothing is embedded.
func TestKeywordOnly(t *testing.T) {
	_, fe, s := setup(t)
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "kb"})
	d, _ := s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "notes", Text: "The fox jumps.\n\nThe owl sleeps."})
	if d.Status != StatusReady || d.Kind != KindText {
		t.Fatalf("add: %+v", d)
	}
	r := mustSearch(t, s, searchv1.SearchInput{Bucket: "kb", Query: "jumping fox"})
	if r.Mode != searchv1.ModeKeyword || len(r.Results) != 1 || fe.calls != 0 {
		t.Fatalf("search: %+v (calls %d)", r, fe.calls)
	}
	if _, err := s.Search(ctx, extkit.Internal, searchv1.SearchInput{Bucket: "kb", Query: "fox", Mode: "semantic"}); err == nil {
		t.Fatal("semantic search without a model")
	}
}

// Limits, removing, deleting, uploads and failures.
func TestLimitsRemoveDelete(t *testing.T) {
	app, fe, s := setup(t)
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "kb", Model: "p/embed"})
	one := 1
	_, _ = s.Update(ctx, extkit.Internal, searchv1.UpdateInput{Name: "kb", MaxDocuments: &one})
	if _, err := s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "a.md", Text: "alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "b.md", Text: "beta"}); err == nil {
		t.Fatal("over max documents")
	}
	if _, err := s.Upload(ctx, extkit.Internal, extkit.RawIn{Query: url.Values{"bucket": {"kb"}, "name": {"x.bin"}},
		Body: strings.NewReader("a\x00b")}); !errors.Is(err, searchv1.ErrNotText) {
		t.Fatalf("binary upload: %v", err)
	}

	fe.fail = llmc.ErrUnknownModel
	drain(t, app)
	// a hybrid search whose query can't be embedded ranks by keywords and says why
	if r := mustSearch(t, s, searchv1.SearchInput{Bucket: "kb", Query: "alpha"}); r.Mode != searchv1.ModeKeyword || r.Note == "" || len(r.Results) != 1 {
		t.Fatalf("fallback: %+v", r)
	}
	docs, _ := s.Documents(ctx, extkit.Internal, searchv1.DocumentsInput{Bucket: "kb"})
	if docs.Documents[0].Status != StatusError || docs.Documents[0].Error == "" || fe.calls != 2 { // the failed embed, then the search's query
		t.Fatalf("a failed embed: %+v (calls %d)", docs, fe.calls)
	}
	fe.fail = nil
	if _, err := s.Reindex(ctx, extkit.Internal, searchv1.RemoveInput{Bucket: "kb", Names: []string{"a.md"}}); err != nil {
		t.Fatal(err)
	}
	drain(t, app)
	if docs, _ = s.Documents(ctx, extkit.Internal, searchv1.DocumentsInput{Bucket: "kb"}); docs.Documents[0].Status != StatusReady {
		t.Fatalf("after reindex: %+v", docs)
	}

	if n, _ := s.Remove(ctx, extkit.Internal, searchv1.RemoveInput{Bucket: "kb", Names: []string{"a.md", "nope"}}); n.Count != 1 {
		t.Fatalf("remove: %+v", n)
	}
	_, _ = s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "b.md", Text: "beta"})
	if _, err := s.Delete(ctx, extkit.Internal, searchv1.Ref{Name: "kb"}); err != nil {
		t.Fatal(err)
	}
	var left int
	_ = app.DB().Select("count(*)").From(FTSTable).Row(&left)
	var chunks int
	_ = app.DB().Select("count(*)").From(ChunksCollection).Row(&chunks)
	if left != 0 || chunks != 0 {
		t.Fatalf("left behind: %d fts rows, %d chunks", left, chunks)
	}
	if _, err := s.Bucket(ctx, extkit.Internal, searchv1.Ref{Name: "kb"}); !errors.Is(err, searchv1.ErrNoBucket) {
		t.Fatalf("deleted bucket: %v", err)
	}
}

const hooks = `onRecordCreate(async (e) => {
  const b = $search.ensure({ name: "storytime", title: "Story Time library", model: "p/embed" })
  const d = $search.add({ bucket: "storytime", name: "fox.md", text: "# Fox\n\nThe red fox naps in the dunes." })
  const r = await $search.searchAsync({ bucket: "storytime", query: "fox", mode: "keyword" })
  e.record.set("result", JSON.stringify({
    title: b.title, status: d.status, buckets: $search.buckets().length, docs: $search.documents("storytime").length,
    heading: r.results[0].heading, removed: $search.remove("storytime", "fox.md"), syncSearch: typeof $search.search,
  }))
  e.next()
}, "probes")
`

func TestJS(t *testing.T) {
	app, _, _ := setup(t)
	c := core.NewBaseCollection("probes")
	c.Fields.Add(&core.TextField{Name: "result", Max: 10000})
	if err := app.Save(c); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "pb_hooks")
	_ = os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, "main.pb.js"), []byte(hooks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := jsvm.Register(app, jsvm.Config{HooksDir: dir, OnInit: func(rt js.Runtime) { New().BindJS(app, rt) }}); err != nil {
		t.Fatal(err)
	}
	if err := app.OnBootstrap().Trigger(&core.BootstrapEvent{App: app}, func(*core.BootstrapEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	r := core.NewRecord(c)
	if err := app.Save(r); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Title, Status, Heading, SyncSearch string
		Buckets, Docs, Removed             int
	}
	if err := json.Unmarshal([]byte(r.GetString("result")), &out); err != nil {
		t.Fatalf("result %q: %v", r.GetString("result"), err)
	}
	if out.Title != "Story Time library" || out.Status != StatusQueued || out.Buckets != 1 || out.Docs != 1 ||
		out.Heading != "Fox" || out.Removed != 1 || out.SyncSearch != "undefined" {
		t.Fatalf("from JS: %+v", out)
	}
}
