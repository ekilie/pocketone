package searchv1

import (
	"context"
	"fmt"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

// EnrichDefault and EnrichMax are how many passages an Enrich adds by default, and at most.
const (
	EnrichDefault = 5
	EnrichMax     = 20
)

// ErrNoSearch: a call asked to enrich its prompt, but nothing provides search/v1 on the app.
var ErrNoSearch = &extkit.Error{Code: "not_configured", Status: 503, Message: "enrich needs search buckets (search/v1), and none are on this app"}

// Enrich runs e's search on app (with query when e.Query is empty) and returns the text a model call sends after its
// prompt, and the passages in it. It's how llm.text/v1 and llm.chat/v1 providers fulfil `enrich`, so every provider
// adds passages the same way. With no buckets named it returns nothing; with no matches, no text.
func Enrich(ctx context.Context, app core.App, e *llm.Enrich, query string) (string, []llm.Passage, error) {
	if e == nil || len(e.Buckets) == 0 {
		return "", nil, nil
	}
	if strings.TrimSpace(e.Query) != "" {
		query = e.Query
	}
	if strings.TrimSpace(query) == "" {
		return "", nil, &extkit.Error{Code: "bad_request", Status: 400, Message: "enrich: no query (give a prompt, or enrich.query)"}
	}
	s, ok := Contract.Of(app)
	if !ok {
		return "", nil, ErrNoSearch
	}
	limit := e.Limit
	if limit <= 0 {
		limit = EnrichDefault
	}
	limit = min(limit, EnrichMax)
	res, err := s.Search(ctx, extkit.Internal, SearchInput{Buckets: e.Buckets, Query: query, Limit: limit})
	if err != nil {
		return "", nil, fmt.Errorf("enrich: %w", err)
	}
	if len(res.Results) == 0 {
		return "", []llm.Passage{}, nil
	}
	var b strings.Builder
	b.WriteString("Passages from the app's own content that may help with this (cite them by number when you use them):")
	passages := make([]llm.Passage, 0, len(res.Results))
	for i, r := range res.Results {
		where := r.Document
		if r.Source != "" && r.Source != r.Document {
			where = r.Source + " › " + where
		}
		if r.Heading != "" {
			where += " › " + r.Heading
		}
		fmt.Fprintf(&b, "\n\n[%d] %s\n%s", i+1, where, r.Text)
		passages = append(passages, llm.Passage{Bucket: r.Bucket, Source: r.Source, Document: r.Document, Heading: r.Heading, Text: r.Text, Score: r.Score})
	}
	return b.String(), passages, nil
}
