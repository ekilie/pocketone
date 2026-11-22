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
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"

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
// close; it remembers every input and fails while fail is set. It lists two models: "p/cheap" and "p/dear".
type fakeEmbed struct {
	mu     sync.Mutex
	inputs []string
	models []string // the model of each input
	calls  int
	fail   error
	none   bool // list no models
}

func (f *fakeEmbed) Embed(_ context.Context, _ extkit.Auth, in embedv1.EmbedInput) (embedv1.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.fail != nil {
		return embedv1.Result{}, f.fail
	}
	res := embedv1.Result{Model: in.Model, Usage: llmc.Usage{CostMicroUSD: int64(len(in.Input))}}
	for _, text := range in.Input {
		f.inputs = append(f.inputs, text)
		f.models = append(f.models, in.Model)
		v := make([]float64, 64)
		for _, w := range wordRe.FindAllString(strings.ToLower(text), -1) {
			h := fnv.New32a()
			_, _ = h.Write([]byte(w))
			v[h.Sum32()%64]++
		}
		res.Vectors = append(res.Vectors, v)
	}
	return res, nil
}

func (f *fakeEmbed) Models(context.Context, extkit.Auth, llmc.ModelsInput) (llmc.Models, error) {
	if f.none {
		return llmc.Models{}, nil
	}
	return llmc.Models{Models: []llmc.ModelInfo{
		{ID: "p/dear", Provider: "p", Kinds: []string{"embed"}, Prices: []llmc.Price{{Unit: "1M input tokens", USD: 0.13}}},
		{ID: "p/cheap", Provider: "p", Kinds: []string{"embed"}, Prices: []llmc.Price{{Unit: "1M input tokens", USD: 0.05}}},
	}}, nil
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
	b, err := s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "storytime", Owner: "Story Time"})
	if err != nil || b.MaxDocuments != DefaultMaxDocuments || b.Owner != "Story Time" {
		t.Fatalf("ensure: %+v %v", b, err)
	}
	b, err = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "storytime", Title: "Library", Owner: "x"})
	if err != nil || b.Title != "Library" || b.Owner != "Story Time" {
		t.Fatalf("ensure again: %+v %v", b, err)
	}
	if _, err := s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "../x"}); !errors.Is(err, searchv1.ErrInvalidName) {
		t.Fatalf("bad name: %v", err)
	}
	if l, _ := s.Buckets(ctx, extkit.Internal); len(l.Buckets) != 1 {
		t.Fatalf("buckets: %+v", l)
	}
}

// A document is split at once and embedded in the background with the app's model (the cheapest one, picked
// automatically); search ranks across the buckets asked for.
func TestAddEmbedSearch(t *testing.T) {
	app, fe, s := setup(t)
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "kb"})
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "other"})
	d, err := s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "stories/lighthouse.md", Text: guide})
	if err != nil || d.Status != StatusQueued || d.Chunks != 3 || d.Kind != KindMarkdown {
		t.Fatalf("add: %+v %v", d, err)
	}
	_, _ = s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "other", Name: "owl.md", Text: "# Owl\n\nThe old owl watches the fox from the tower."})
	drain(t, app)
	docs, _ := s.Documents(ctx, extkit.Internal, searchv1.DocumentsInput{Bucket: "kb"})
	if len(docs.Documents) != 1 || docs.Documents[0].Status != StatusReady || docs.Documents[0].CostMicroUSD != 3 {
		t.Fatalf("after embedding: %+v", docs)
	}
	if fe.embedded() != 4 || fe.models[0] != "p/cheap" || !strings.HasPrefix(fe.inputs[1], "stories/lighthouse.md — Lighthouse › The fox\n\n") {
		t.Fatalf("embedded: %q with %q", fe.inputs, fe.models)
	}
	if st, _ := s.Status(ctx, extkit.Internal); st.Model != "p/cheap" || st.Chunks != 4 || st.Embedded != 4 {
		t.Fatalf("status: %+v", st)
	}

	r := mustSearch(t, s, searchv1.SearchInput{Buckets: []string{"kb"}, Query: "red fox dunes", Limit: 2})
	if len(r.Results) == 0 || r.Results[0].Heading != "Lighthouse › The fox" || r.Results[0].Bucket != "kb" || r.Model != "p/cheap" {
		t.Fatalf("one bucket: %+v", r)
	}
	all := mustSearch(t, s, searchv1.SearchInput{Query: "fox", Limit: 10})
	got := map[string]bool{}
	for _, x := range all.Results {
		got[x.Bucket] = true
	}
	if !got["kb"] || !got["other"] {
		t.Fatalf("every bucket: %+v", all)
	}
	if r := mustSearch(t, s, searchv1.SearchInput{Bucket: "kb", Query: "fog", Prefix: "other/"}); len(r.Results) != 0 {
		t.Fatalf("prefix: %+v", r)
	}
	if _, err := s.Search(ctx, extkit.Internal, searchv1.SearchInput{Buckets: []string{"nope"}, Query: "fox"}); err == nil {
		t.Fatal("an unknown bucket was searched")
	}
	// FTS operators in a query are words, not syntax
	if _, err := s.Search(ctx, extkit.Internal, searchv1.SearchInput{Bucket: "kb", Query: `fox" OR NEAR(*`}); err != nil {
		t.Fatal(err)
	}
	b, _ := s.Bucket(ctx, extkit.Internal, searchv1.Ref{Name: "kb"})
	if b.Documents != 1 || b.Chunks != 3 || b.Pending != 0 || b.Bytes != int64(len(guide)) {
		t.Fatalf("bucket: %+v", b)
	}
}

// The same text again costs nothing; a changed document embeds only its new chunks; a new model embeds everything.
func TestReingestAndModel(t *testing.T) {
	app, fe, s := setup(t)
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "kb"})
	_, _ = s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "a.md", Text: guide})
	drain(t, app)
	d, err := s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "a.md", Text: guide})
	if err != nil || !d.Unchanged || d.Status != StatusReady {
		t.Fatalf("again: %+v %v", d, err)
	}
	changed := strings.Replace(guide, "follow the beam", "trust the beam", 1)
	if d, _ = s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "a.md", Text: changed}); d.Status != StatusQueued {
		t.Fatalf("changed: %+v", d)
	}
	drain(t, app)
	if fe.embedded() != 4 || !strings.Contains(fe.inputs[3], "trust the beam") {
		t.Fatalf("re-embedded more than the changed chunk: %q", fe.inputs)
	}
	if r := mustSearch(t, s, searchv1.SearchInput{Bucket: "kb", Query: "trust", Limit: 1}); len(r.Results) != 1 || !strings.Contains(r.Results[0].Text, "trust") {
		t.Fatalf("new text: %+v", r)
	}

	st, err := s.SetModel(ctx, extkit.Internal, searchv1.ModelInput{Model: "p/dear"})
	if err != nil || st.Model != "p/dear" || st.Embedded != 0 {
		t.Fatalf("set model: %+v %v", st, err)
	}
	// until it's embedded again, keywords still find it
	if r := mustSearch(t, s, searchv1.SearchInput{Bucket: "kb", Query: "trust", Limit: 1}); len(r.Results) != 1 {
		t.Fatalf("during the change: %+v", r)
	}
	drain(t, app)
	// 4 chunks and a query with p/cheap, a query with p/dear during the change, then the 3 chunks
	if fe.embedded() != 9 || fe.models[8] != "p/dear" {
		t.Fatalf("after a model change: %d embedded, %q", fe.embedded(), fe.models)
	}
	if st, _ := s.Status(ctx, extkit.Internal); st.Embedded != 3 {
		t.Fatalf("status after: %+v", st)
	}
	if n, err := s.Reindex(ctx, extkit.Internal, searchv1.RemoveInput{Bucket: "kb"}); err != nil || n.Count != 1 {
		t.Fatalf("reindex: %+v %v", n, err)
	}
	if _, err := s.SetModel(ctx, extkit.Internal, searchv1.ModelInput{Model: "nomodel"}); err == nil {
		t.Fatal("a model without a provider was accepted")
	}
}

// Without an embedding model documents wait, search says what's missing, and nothing is lost.
func TestNoModel(t *testing.T) {
	app, fe, s := setup(t)
	fe.none = true
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "kb"})
	d, _ := s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "notes", Text: "The fox jumps.\n\nThe owl sleeps."})
	drain(t, app)
	if docs, _ := s.Documents(ctx, extkit.Internal, searchv1.DocumentsInput{Bucket: "kb"}); docs.Documents[0].Status != StatusQueued || d.Kind != KindText {
		t.Fatalf("waiting: %+v", docs)
	}
	if _, err := s.Search(ctx, extkit.Internal, searchv1.SearchInput{Bucket: "kb", Query: "fox"}); !errors.Is(err, searchv1.ErrNoModel) {
		t.Fatalf("search: %v", err)
	}
	if st, _ := s.Status(ctx, extkit.Internal); st.Model != "" || st.Note == "" {
		t.Fatalf("status: %+v", st)
	}
	fe.none = false // a provider is added
	drain(t, app)
	if r := mustSearch(t, s, searchv1.SearchInput{Bucket: "kb", Query: "fox"}); len(r.Results) != 1 || fe.embedded() != 2 {
		t.Fatalf("after a provider: %+v", r)
	}
}

// Limits, removing, deleting, uploads and failures.
func TestLimitsRemoveDelete(t *testing.T) {
	app, fe, s := setup(t)
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "kb"})
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
	docs, _ := s.Documents(ctx, extkit.Internal, searchv1.DocumentsInput{Bucket: "kb"})
	if docs.Documents[0].Status != StatusError || docs.Documents[0].Error == "" || fe.calls != 1 {
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
	var left, chunks int
	_ = app.DB().Select("count(*)").From(FTSTable).Row(&left)
	_ = app.DB().Select("count(*)").From(ChunksCollection).Row(&chunks)
	if left != 0 || chunks != 0 {
		t.Fatalf("left behind: %d fts rows, %d chunks", left, chunks)
	}
	if _, err := s.Bucket(ctx, extkit.Internal, searchv1.Ref{Name: "kb"}); !errors.Is(err, searchv1.ErrNoBucket) {
		t.Fatalf("deleted bucket: %v", err)
	}
}

// A bucket that follows a collection keeps its records in sync, and a search for a viewer leaves out what they can't
// view.
func TestFollowCollection(t *testing.T) {
	app, _, s := setup(t)
	members := core.NewAuthCollection("members")
	if err := app.Save(members); err != nil {
		t.Fatal(err)
	}
	notes := core.NewBaseCollection("notes")
	rule := "owner = @request.auth.id"
	notes.ViewRule = types.Pointer(rule)
	notes.Fields.Add(&core.TextField{Name: "title"}, &core.EditorField{Name: "body"}, &core.BoolField{Name: "published"},
		&core.RelationField{Name: "owner", CollectionId: members.Id, MaxSelect: 1})
	if err := app.Save(notes); err != nil {
		t.Fatal(err)
	}
	member := func(email string) *core.Record {
		r := core.NewRecord(members)
		r.SetEmail(email)
		r.SetPassword("password123")
		if err := app.Save(r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	ann, bob := member("ann@example.com"), member("bob@example.com")
	note := func(title, body, owner string, published bool) *core.Record {
		r := core.NewRecord(notes)
		r.Set("title", title)
		r.Set("body", body)
		r.Set("owner", owner)
		r.Set("published", published)
		if err := app.Save(r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	n1 := note("Fox facts", "<p>The <b>red fox</b> hunts at dawn.</p>", ann.Id, true) // before the bucket follows
	if _, err := s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "notes",
		Source: &searchv1.Source{Collection: "notes", Fields: []string{"title", "body"}, Filter: "published = true"}}); err != nil {
		t.Fatal(err)
	}
	n2 := note("Fox dens", "<p>A fox den has many exits.</p>", bob.Id, true)
	_ = note("Fox secrets", "<p>Draft about the fox.</p>", ann.Id, false) // filtered out
	drain(t, app)

	docs, _ := s.Documents(ctx, extkit.Internal, searchv1.DocumentsInput{Bucket: "notes"})
	if len(docs.Documents) != 2 || docs.Documents[0].Record == nil || docs.Documents[0].Kind != KindMarkdown {
		t.Fatalf("documents: %+v", docs)
	}
	r := mustSearch(t, s, searchv1.SearchInput{Bucket: "notes", Query: "red fox hunts", Limit: 1})
	if len(r.Results) != 1 || r.Results[0].Record.ID != n1.Id || r.Results[0].Heading != "Fox facts" ||
		strings.Contains(r.Results[0].Text, "<b>") {
		t.Fatalf("search: %+v", r)
	}

	ids := func(r searchv1.Results) string {
		var out []string
		for _, x := range r.Results {
			out = append(out, x.Record.ID)
		}
		return strings.Join(out, ",")
	}
	all := mustSearch(t, s, searchv1.SearchInput{Bucket: "notes", Query: "fox"})
	asAnn := mustSearch(t, s, searchv1.SearchInput{Bucket: "notes", Query: "fox", Viewer: &searchv1.Viewer{Collection: "members", ID: ann.Id}})
	asGuest := mustSearch(t, s, searchv1.SearchInput{Bucket: "notes", Query: "fox", Viewer: &searchv1.Viewer{}})
	if len(all.Results) != 2 || ids(asAnn) != n1.Id || len(asGuest.Results) != 0 {
		t.Fatalf("viewers: all %s, ann %s, guest %s", ids(all), ids(asAnn), ids(asGuest))
	}

	// edits follow; unpublishing and deleting remove
	n1.Set("body", "<p>The red fox naps in the sun.</p>")
	_ = app.Save(n1)
	drain(t, app)
	if r := mustSearch(t, s, searchv1.SearchInput{Bucket: "notes", Query: "naps sun", Limit: 1}); len(r.Results) != 1 || !strings.Contains(r.Results[0].Text, "naps") {
		t.Fatalf("after an edit: %+v", r)
	}
	n1.Set("published", false)
	_ = app.Save(n1)
	_ = app.Delete(n2)
	if docs, _ := s.Documents(ctx, extkit.Internal, searchv1.DocumentsInput{Bucket: "notes"}); len(docs.Documents) != 0 {
		t.Fatalf("after unpublish and delete: %+v", docs)
	}
	if _, err := s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "bad", Source: &searchv1.Source{Collection: "notes", Fields: []string{"nope"}}}); err == nil {
		t.Fatal("a source with a missing field was accepted")
	}
}

// Enrich searches the buckets and words the passages for a prompt.
func TestEnrich(t *testing.T) {
	app, _, s := setup(t)
	_, _ = s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "kb"})
	_, _ = s.Add(ctx, extkit.Internal, searchv1.AddInput{Bucket: "kb", Name: "a.md", Text: guide})
	drain(t, app)
	var e llmc.Enrich
	if err := json.Unmarshal([]byte(`["kb"]`), &e); err != nil || len(e.Buckets) != 1 {
		t.Fatalf("shorthand: %+v %v", e, err)
	}
	text, passages, err := searchv1.Enrich(ctx, app, &llmc.Enrich{Buckets: []string{"kb"}, Limit: 2}, "where does the fox live?")
	if err != nil || len(passages) != 2 || passages[0].Heading != "Lighthouse › The fox" || !strings.Contains(text, "[1] a.md › Lighthouse › The fox\nA red fox") {
		t.Fatalf("enrich: %q %+v %v", text, passages, err)
	}
	if text, p, err := searchv1.Enrich(ctx, app, nil, "x"); text != "" || p != nil || err != nil {
		t.Fatal("enrich without buckets did something")
	}
}

const hooks = `onRecordCreate(async (e) => {
  const b = $search.ensure({ name: "storytime", title: "Story Time library" })
  const d = $search.add({ bucket: "storytime", name: "fox.md", text: "# Fox\n\nThe red fox naps in the dunes." })
  const r = await $search.searchAsync({ buckets: "storytime", query: "fox", viewer: null })
  e.record.set("result", JSON.stringify({
    title: b.title, status: d.status, buckets: $search.buckets().length, docs: $search.documents("storytime").length,
    heading: r.results[0].heading, model: $search.status().model, removed: $search.remove("storytime", "fox.md"),
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
		Title, Status, Heading, Model string
		Buckets, Docs, Removed        int
	}
	if err := json.Unmarshal([]byte(r.GetString("result")), &out); err != nil {
		t.Fatalf("result %q: %v", r.GetString("result"), err)
	}
	if out.Title != "Story Time library" || out.Status != StatusQueued || out.Buckets != 1 || out.Docs != 1 ||
		out.Heading != "Fox" || out.Model != "p/cheap" || out.Removed != 1 {
		t.Fatalf("from JS: %+v", out)
	}
}
