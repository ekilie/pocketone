package llmchat_test

import (
	"context"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"

	llmc "pocketone.dev/ext/contracts/llm"
	chatv1 "pocketone.dev/ext/contracts/llm/chat/v1"
	searchv1 "pocketone.dev/ext/contracts/search/v1"
	"pocketone.dev/ext/extkit"
	llmchat "pocketone.dev/ext/llm-chat"
)

type searchBase = searchv1.Search

// fakeSearch answers every search with one passage.
type fakeSearch struct{ searchBase }

func (fakeSearch) Search(_ context.Context, _ extkit.Auth, in searchv1.SearchInput) (searchv1.Results, error) {
	return searchv1.Results{Results: []searchv1.Result{{Bucket: in.Buckets[0], Document: "fox.md", Text: "The fox naps in the dunes.", Score: 1}}}, nil
}

type fakeSearchExt struct{}

func (fakeSearchExt) Name() string { return "fake-search" }
func (fakeSearchExt) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("search/v1")}
}
func (fakeSearchExt) Register(app core.App, _ extkit.Config) error {
	searchv1.Contract.Provide(app, fakeSearch{})
	return nil
}

// A send's enrich adds passages after the question (saved with it, so the next turn sends them again), and the answer
// lists them.
func TestEnrich(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	fa := &fakeText{}
	if _, err := extkit.Setup(app, []extkit.Extension{fakeExt{fa}, fakeSearchExt{}, llmchat.New()}, nil, nil); err != nil {
		t.Fatal(err)
	}
	c := chatv1.Contract.Must(app)
	ctx := context.Background()
	chat, _ := c.New(ctx, extkit.Internal, chatv1.NewInput{})
	r, err := c.Send(ctx, extkit.Internal, chatv1.SendInput{ID: chat.ID, Text: "Where does the fox sleep?", Model: "p/m",
		Enrich: &llmc.Enrich{Buckets: []string{"stories"}}})
	if err != nil || len(r.Answer.Passages) != 1 {
		t.Fatalf("send: %+v %v", r, err)
	}
	q := fa.last().Messages[len(fa.last().Messages)-1].Content
	if !strings.HasPrefix(q, "Where does the fox sleep?\n\n") || !strings.Contains(q, "[1] fox.md\nThe fox naps") {
		t.Fatalf("sent: %q", q)
	}
	_, _ = c.Send(ctx, extkit.Internal, chatv1.SendInput{ID: chat.ID, Text: "And by day?", Model: "p/m"})
	if first := fa.last().Messages[0].Content; first != q {
		t.Fatalf("the next turn didn't send the enriched question as before: %q", first)
	}
	if h, _ := c.History(ctx, extkit.Internal, chatv1.HistoryInput{ID: chat.ID}); h.Messages[0].Content != "Where does the fox sleep?" {
		t.Fatalf("history shows the passages: %q", h.Messages[0].Content)
	}
}
