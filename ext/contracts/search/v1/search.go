// Package searchv1 is the search/v1 contract: named buckets of content an app searches. An app makes the bucket it
// needs (Ensure) and searches it; the app's admins load content into it (files or pasted text) from the dashboard's
// Search screen, or the bucket follows one of the app's collections and keeps itself in sync. The provider splits
// each document into chunks with their source (document, section, lines, record), embeds them with the app's one
// embedding model (llm.embed/v1, picked automatically), and returns the chunks that best match a query across the
// buckets asked for.
//
// A wire contract at /api/_c/search/v1. In page code it's $search; llm.text/v1 and llm.chat/v1 calls take `enrich` to
// add a search's best passages to their prompt ([Enrich]).
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
	// Status is the app's embedding model and how far embedding has got across every bucket.
	//
	//op: GET status scope=search:read
	Status(ctx context.Context, a extkit.Auth) (Status, error)
	// SetModel changes the app's embedding model ("provider/model" from llm.embed/v1's models; "" picks one again).
	// Every bucket is embedded again in the background; until a chunk is, keywords still find it.
	//
	//op: POST set-model
	SetModel(ctx context.Context, a extkit.Auth, in ModelInput) (Status, error)
	// Buckets lists the app's buckets with their counts.
	//
	//op: GET buckets scope=search:read
	Buckets(ctx context.Context, a extkit.Auth) (BucketList, error)
	// Bucket returns one bucket with its counts.
	//
	//op: GET bucket scope=search:read
	Bucket(ctx context.Context, a extkit.Auth, in Ref) (Bucket, error)
	// Ensure makes a bucket if it's missing and returns it. An existing bucket keeps its settings, except that an
	// empty title, description, owner or source is filled in. Apps call it at startup for the bucket they need.
	//
	//op: POST ensure
	Ensure(ctx context.Context, a extkit.Auth, in EnsureInput) (Bucket, error)
	// Update changes a bucket's title, description, source or limits (fields left out stay).
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
	// Reindex splits and embeds a bucket's documents again (or only the named ones), e.g. after an error; a bucket
	// that follows a collection reads its records again.
	//
	//op: POST reindex
	Reindex(ctx context.Context, a extkit.Auth, in RemoveInput) (Removed, error)
	// Search returns the chunks that best match a query across the buckets named, best first.
	//
	//op: POST search scope=search:read
	Search(ctx context.Context, a extkit.Auth, in SearchInput) (Results, error)
}

// Status is the app's embedding model and its progress.
type Status struct {
	// Model is the embedding model every bucket uses ("provider/model"); "" until one is picked (none available yet).
	Model string `json:"model"`
	// Chunks is how many chunks the app's buckets hold; Embedded how many have the model's vectors.
	Chunks   int `json:"chunks"`
	Embedded int `json:"embedded"`
	// Note says what's missing when nothing can be embedded (no embedding model among the app's providers).
	Note string `json:"note,omitempty"`
}

// ModelInput picks the app's embedding model.
type ModelInput struct {
	Model string `json:"model"`
}

// Ref names a bucket.
type Ref struct {
	Name string `json:"name"`
}

// Source makes a bucket follow one of the app's collections: each record (that Filter keeps) is a document named
// "<collection>/<record id>", made of Fields (the first one its title), added, replaced and removed as records are.
type Source struct {
	Collection string   `json:"collection"`
	Fields     []string `json:"fields"`
	Filter     string   `json:"filter,omitempty"` // a PocketBase filter, e.g. "published = true"
}

// Bucket is a named set of documents searched together.
type Bucket struct {
	// Name is the bucket's id: letters, digits, - and _, starting with a letter or digit (at most 64).
	Name  string `json:"name"`
	Title string `json:"title"`
	// Description says what belongs in the bucket: the app that made it tells its admins what to load.
	Description string `json:"description"`
	// Owner is who made it (the app's name, say), for the dashboard.
	Owner        string    `json:"owner"`
	Source       *Source   `json:"source,omitempty"`
	MaxDocuments int       `json:"maxDocuments"`
	MaxBytes     int64     `json:"maxBytes"`
	Documents    int       `json:"documents"`
	Chunks       int       `json:"chunks"`
	Bytes        int64     `json:"bytes"`
	Pending      int       `json:"pending"` // documents waiting to be embedded
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
	Name        string  `json:"name"`
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Owner       string  `json:"owner,omitempty"`
	Source      *Source `json:"source,omitempty"`
}

// UpdateInput changes a bucket. A Source with no collection stops following one (its documents stay).
type UpdateInput struct {
	Name         string  `json:"name"`
	Title        *string `json:"title,omitempty"`
	Description  *string `json:"description,omitempty"`
	Source       *Source `json:"source,omitempty"`
	MaxDocuments *int    `json:"maxDocuments,omitempty"`
	MaxBytes     *int64  `json:"maxBytes,omitempty"`
}

// AddInput is a document by name and text.
type AddInput struct {
	Bucket string `json:"bucket"`
	// Name is the document's name in the bucket, usually its path ("guides/install.md"). Its extension picks the
	// splitter: Markdown by headings, code and the rest by blank lines.
	Name string `json:"name"`
	Text string `json:"text"`
}

// Document is one file, text or record in a bucket.
type Document struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
	Kind   string `json:"kind"` // markdown, code or text
	Size   int64  `json:"size"`
	Hash   string `json:"hash"` // sha256 of the text
	// Status is queued (waiting to be embedded), embedding, ready or error.
	Status string     `json:"status"`
	Error  string     `json:"error,omitempty"`
	Chunks int        `json:"chunks"`
	Record *RecordRef `json:"record,omitempty"` // the record it was made from, in a bucket that follows a collection
	// Tokens is the estimated size of the text in tokens; CostMicroUSD what embedding it has cost so far.
	Tokens       int       `json:"tokens"`
	CostMicroUSD int64     `json:"costMicroUSD"`
	Unchanged    bool      `json:"unchanged,omitempty"` // Add: the same text was already there
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
}

// RecordRef names a record.
type RecordRef struct {
	Collection string `json:"collection"`
	ID         string `json:"id"`
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

// Viewer is who a search is for. Matches from records they can't view (the collection's view rule, as PocketBase
// checks it) are left out; documents that aren't records are for everyone. A search without one sees everything.
type Viewer struct {
	// Collection and ID are the signed-in auth record; both empty is a guest (signed out).
	Collection string `json:"collection,omitempty"`
	ID         string `json:"id,omitempty"`
}

// SearchInput is a query.
type SearchInput struct {
	// Buckets are searched together; none searches every bucket. Bucket is one more, for a single-bucket search.
	Buckets []string `json:"buckets,omitempty"`
	Bucket  string   `json:"bucket,omitempty"`
	Query   string   `json:"query"`
	Limit   int      `json:"limit,omitempty"` // default 5, at most 50
	// Prefix keeps matches from documents whose name starts with it.
	Prefix string  `json:"prefix,omitempty"`
	Viewer *Viewer `json:"viewer,omitempty"`
}

// Results are the best matches, best first.
type Results struct {
	Results []Result  `json:"results"`
	Model   string    `json:"model,omitempty"` // the model that embedded the query
	Usage   llm.Usage `json:"usage"`           // what embedding the query cost
}

// Result is one matching chunk and where it came from.
type Result struct {
	Bucket   string `json:"bucket"`
	Document string `json:"document"`
	Text     string `json:"text"`
	// Heading is the section the chunk is in ("Install › macOS"), for Markdown and records.
	Heading   string     `json:"heading,omitempty"`
	StartLine int        `json:"startLine"`
	EndLine   int        `json:"endLine"`
	Record    *RecordRef `json:"record,omitempty"`
	Score     float64    `json:"score"` // orders the results; higher is better
}

// Errors.
var (
	ErrNoBucket    = &extkit.Error{Code: "not_found", Status: 404, Message: "no bucket with that name"}
	ErrNoDocument  = &extkit.Error{Code: "not_found", Status: 404, Message: "no document with that name"}
	ErrInvalidName = &extkit.Error{Code: "bad_request", Status: 400, Message: "a bucket name is letters, digits, - and _, at most 64, starting with a letter or digit"}
	ErrTooLarge    = &extkit.Error{Code: "too_large", Status: 413, Message: "over the bucket's limit"}
	ErrNotText     = &extkit.Error{Code: "bad_request", Status: 400, Message: "only text files (Markdown, text, code) can be added"}
	ErrNoModel     = &extkit.Error{Code: "not_configured", Status: 503, Message: "no embedding model: add an AI provider with embedding models in Settings → LLM"}
)
