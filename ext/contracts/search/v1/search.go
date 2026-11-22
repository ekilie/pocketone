// Package searchv1 is the search/v1 contract: named buckets of content an app searches. An app makes the bucket it
// needs (Ensure) and searches it; the app's admins load content into it (files or pasted text) from the dashboard's
// Search screen, the CLI or an agent. The provider splits each document into chunks with their source (document,
// section, lines), embeds them through llm.embed/v1 when the bucket has a model, and ranks matches by meaning and by
// keywords together.
//
// A wire contract at /api/_c/search/v1. In page code it's $search.
package searchv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"
	"time"

	"pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

// Search keeps an app's search buckets and searches them.
//
//contract:id search/v1
//contract:kind wire
//contract:scope search
//contract:js hooks
type Search interface {
	// Buckets lists the app's buckets with their counts.
	//
	//op: GET buckets scope=search:read
	Buckets(ctx context.Context, a extkit.Auth) (BucketList, error)
	// Bucket returns one bucket with its counts.
	//
	//op: GET bucket scope=search:read
	Bucket(ctx context.Context, a extkit.Auth, in Ref) (Bucket, error)
	// Ensure makes a bucket if it's missing and returns it. An existing bucket keeps its settings, except that an
	// empty title or description is filled in. Apps call it at startup for the bucket they need.
	//
	//op: POST ensure
	Ensure(ctx context.Context, a extkit.Auth, in EnsureInput) (Bucket, error)
	// Update changes a bucket's title, description, model or limits (fields left out stay). A new model or dimensions
	// embeds every chunk again.
	//
	//op: POST update
	Update(ctx context.Context, a extkit.Auth, in UpdateInput) (Bucket, error)
	// Delete removes a bucket and everything in it.
	//
	//op: POST delete
	Delete(ctx context.Context, a extkit.Auth, in Ref) (Removed, error)
	// Add puts a document in a bucket from its text, or replaces the document of that name. An unchanged document costs
	// nothing; a changed one embeds only its new chunks. It returns at once; the document is embedded in the background
	// (its status says how far it got).
	//
	//op: POST add
	Add(ctx context.Context, a extkit.Auth, in AddInput) (Document, error)
	// Upload is Add with the file as the request body: ?bucket=<name>&name=<path>.
	//
	//op: POST upload
	Upload(ctx context.Context, a extkit.Auth, in extkit.RawIn) (Document, error)
	// Documents lists a bucket's documents, by name.
	//
	//op: GET documents scope=search:read
	Documents(ctx context.Context, a extkit.Auth, in DocumentsInput) (DocumentList, error)
	// Remove deletes documents from a bucket by name.
	//
	//op: POST remove
	Remove(ctx context.Context, a extkit.Auth, in RemoveInput) (Removed, error)
	// Reindex splits and embeds a bucket's documents again (or only the named ones), e.g. after an error.
	//
	//op: POST reindex
	Reindex(ctx context.Context, a extkit.Auth, in RemoveInput) (Removed, error)
	// Search returns the chunks of a bucket that best match a query, best first.
	//
	//op: POST search scope=search:read
	Search(ctx context.Context, a extkit.Auth, in SearchInput) (Results, error)
}

// Ref names a bucket.
type Ref struct {
	Name string `json:"name"`
}

// Bucket is a named set of documents searched together.
type Bucket struct {
	// Name is the bucket's id: letters, digits, - and _, starting with a letter or digit (at most 64).
	Name  string `json:"name"`
	Title string `json:"title"`
	// Description says what belongs in the bucket: the app that made it tells its admins what to load.
	Description string `json:"description"`
	// Owner is who made it (the app's name, say), for the dashboard.
	Owner string `json:"owner"`
	// Model is the embedding model ("provider/model" from Settings → LLM); "" searches by keywords only.
	Model string `json:"model"`
	// Dimensions shortens the model's vectors (0 keeps the model's own length).
	Dimensions   int       `json:"dimensions"`
	MaxDocuments int       `json:"maxDocuments"`
	MaxBytes     int64     `json:"maxBytes"`
	Documents    int       `json:"documents"`
	Chunks       int       `json:"chunks"`
	Bytes        int64     `json:"bytes"`
	Pending      int       `json:"pending"` // documents queued or being embedded
	Errors       int       `json:"errors"`  // documents that failed
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
}

// BucketList is every bucket, by name.
type BucketList struct {
	Buckets []Bucket `json:"buckets"`
}

// EnsureInput is the bucket an app needs.
type EnsureInput struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Owner       string `json:"owner,omitempty"`
	// Model and Dimensions apply only when the bucket is made; the admin can change them later.
	Model      string `json:"model,omitempty"`
	Dimensions int    `json:"dimensions,omitempty"`
}

// UpdateInput changes a bucket.
type UpdateInput struct {
	Name         string  `json:"name"`
	Title        *string `json:"title,omitempty"`
	Description  *string `json:"description,omitempty"`
	Model        *string `json:"model,omitempty"`
	Dimensions   *int    `json:"dimensions,omitempty"`
	MaxDocuments *int    `json:"maxDocuments,omitempty"`
	MaxBytes     *int64  `json:"maxBytes,omitempty"`
}

// AddInput is a document by name and text.
type AddInput struct {
	Bucket string `json:"bucket"`
	// Name is the document's name in the bucket, usually its path ("guides/install.md"). Its extension picks the
	// splitter: Markdown by headings, code by blocks, the rest by paragraphs.
	Name string `json:"name"`
	Text string `json:"text"`
}

// Document is one file or text in a bucket.
type Document struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
	Kind   string `json:"kind"` // markdown, code or text
	Size   int64  `json:"size"`
	Hash   string `json:"hash"` // sha256 of the text
	// Status is queued, embedding, ready or error.
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Chunks int    `json:"chunks"`
	// Tokens is the estimated size of the text in tokens; CostMicroUSD what embedding it has cost so far.
	Tokens       int       `json:"tokens"`
	CostMicroUSD int64     `json:"costMicroUSD"`
	Unchanged    bool      `json:"unchanged,omitempty"` // Add: the same text was already there
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
}

// DocumentsInput picks a bucket's documents.
type DocumentsInput struct {
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix,omitempty"` // only names starting with it
}

// DocumentList is a bucket's documents.
type DocumentList struct {
	Documents []Document `json:"documents"`
}

// RemoveInput names documents (none: all of them, for Reindex).
type RemoveInput struct {
	Bucket string   `json:"bucket"`
	Names  []string `json:"names,omitempty"`
}

// Removed says how many things a call affected.
type Removed struct {
	Count int `json:"count"`
}

// Search modes.
const (
	ModeHybrid   = "hybrid"   // by meaning and keywords together (the default; keywords only without a model)
	ModeSemantic = "semantic" // by meaning only
	ModeKeyword  = "keyword"  // by keywords only (no model call)
)

// SearchInput is a query.
type SearchInput struct {
	Bucket string `json:"bucket"`
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"` // default 5, at most 50
	Mode   string `json:"mode,omitempty"`
	// Prefix keeps matches from documents whose name starts with it.
	Prefix string `json:"prefix,omitempty"`
}

// Results are the best matches, best first.
type Results struct {
	Results []Result  `json:"results"`
	Mode    string    `json:"mode"`            // the mode that ran
	Model   string    `json:"model,omitempty"` // the model that embedded the query
	Usage   llm.Usage `json:"usage"`           // what embedding the query cost
	// Note says why a hybrid search ranked by keywords only (the model couldn't embed the query).
	Note string `json:"note,omitempty"`
}

// Result is one matching chunk and where it came from.
type Result struct {
	Text     string `json:"text"`
	Document string `json:"document"`
	// Heading is the section the chunk is in ("Install › macOS"), for Markdown.
	Heading   string `json:"heading,omitempty"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	// Score orders the results (higher is better); Semantic is the cosine similarity and Keyword the BM25 score,
	// each when that half matched.
	Score    float64 `json:"score"`
	Semantic float64 `json:"semantic,omitempty"`
	Keyword  float64 `json:"keyword,omitempty"`
}

// Errors.
var (
	ErrNoBucket    = &extkit.Error{Code: "not_found", Status: 404, Message: "no bucket with that name"}
	ErrNoDocument  = &extkit.Error{Code: "not_found", Status: 404, Message: "no document with that name"}
	ErrInvalidName = &extkit.Error{Code: "bad_request", Status: 400, Message: "a bucket name is letters, digits, - and _, at most 64, starting with a letter or digit"}
	ErrTooLarge    = &extkit.Error{Code: "too_large", Status: 413, Message: "over the bucket's limit"}
	ErrNotText     = &extkit.Error{Code: "bad_request", Status: 400, Message: "only text files (Markdown, text, code) can be added"}
)
