package llm_test

import (
	"context"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"

	llmc "pocketone.dev/ext/contracts/llm"
	textv1 "pocketone.dev/ext/contracts/llm/text/v1"
	searchv1 "pocketone.dev/ext/contracts/search/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/llm"
)

// fakeSearch answers every search with one passage from the bucket asked for.
type fakeSearch struct {
	searchBase // the operations it doesn't answer (never called)
	got        []searchv1.SearchInput
}

type searchBase = searchv1.Search

func (f *fakeSearch) Search(_ context.Context, _ extkit.Auth, in searchv1.SearchInput) (searchv1.Results, error) {
	f.got = append(f.got, in)
	return searchv1.Results{Results: []searchv1.Result{{Bucket: in.Buckets[0], Document: "fox.md", Heading: "Fox", Text: "The fox naps.", Score: 1}}}, nil
}

type fakeSearchExt struct{ f *fakeSearch }

func (fakeSearchExt) Name() string { return "fake-search" }
func (fakeSearchExt) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("search/v1")}
}
func (x fakeSearchExt) Register(app core.App, _ extkit.Config) error {
	searchv1.Contract.Provide(app, x.f)
	return nil
}

// enrich searches the buckets with the prompt and sends the passages after it; the answer lists them.
func TestEnrich(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	fs := &fakeSearch{}
	if _, err := extkit.Setup(app, []extkit.Extension{llm.New(), fakeSearchExt{fs}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	fc := withFakeCloud(t, app)
	r, err := textv1.Contract.Must(app).Complete(context.Background(), extkit.Internal, textv1.CompleteInput{
		Model: "fake/chat-1", Prompt: "Where does the fox sleep?", Enrich: &llmc.Enrich{Buckets: []string{"stories"}, Limit: 3}})
	if err != nil || len(r.Passages) != 1 || r.Passages[0].Document != "fox.md" {
		t.Fatalf("complete: %+v %v", r, err)
	}
	if len(fs.got) != 1 || fs.got[0].Query != "Where does the fox sleep?" || fs.got[0].Limit != 3 {
		t.Fatalf("searched: %+v", fs.got)
	}
	msgs := fc.body("/v1/chat/completions")["messages"].([]any)
	last := msgs[len(msgs)-1].(map[string]any)["content"].(string)
	if !strings.HasPrefix(last, "Where does the fox sleep?\n\n") || !strings.Contains(last, "[1] fox.md › Fox\nThe fox naps.") {
		t.Fatalf("sent: %q", last)
	}
}
