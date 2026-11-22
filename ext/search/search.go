// Package search provides search/v1: named buckets of content an app searches. An app makes the bucket it needs
// ($search.ensure) and searches it ($search.searchAsync); its admins load files and text into it from the dashboard's
// Search screen. In page code it's $search.
//
// A bucket is a record in _search_buckets; each document (a file or pasted text, by name) is in _search_documents
// with its text kept as a file; its chunks are in _search_chunks, with their heading path, lines and embedding, and
// in the FTS5 table _search_fts for keyword ranking. Adding a document splits it at once; a background worker embeds
// the chunks through llm.embed/v1. Search ranks by cosine similarity (a loop over the bucket's vectors, cached in
// memory) and BM25 together, merged by reciprocal rank.
package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/filesystem"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/security"

	searchv1 "pocketone.dev/ext/contracts/search/v1"
	"pocketone.dev/ext/extkit"
)

// Name is the extension's key.
const Name = "search"

// Limits a new bucket starts with; its admins can change them.
const (
	DefaultMaxDocuments = 2000
	DefaultMaxBytes     = 50 << 20
)

// Document statuses.
const (
	StatusQueued    = "queued"
	StatusEmbedding = "embedding"
	StatusReady     = "ready"
	StatusError     = "error"
)

// Extension provides search/v1 on every app it's registered on.
type Extension struct{}

var (
	_ extkit.Extension = (*Extension)(nil)
	_ extkit.Migrator  = (*Extension)(nil)
	_ extkit.JSBinder  = (*Extension)(nil)
)

// New returns the extension.
func New() *Extension { return &Extension{} }

// Name implements extkit.Extension.
func (*Extension) Name() string { return Name }

// Contracts implements extkit.Declarer: it embeds through llm.embed/v1 when a bucket has a model, and searches by
// keywords alone without one.
func (*Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("search/v1"), WorksWith: extkit.IDs("llm.embed/v1")}
}

// Register implements extkit.Extension.
func (*Extension) Register(app core.App, _ extkit.Config) error {
	s := newService(app)
	app.Store().Set(storeKey, s)
	searchv1.Contract.Provide(app, s)
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		s.kickWorker() // resume what was queued before a restart
		return e.Next()
	})
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		s.stopWorker()
		return e.Next()
	})
	return nil
}

const storeKey = "pocketone.search"

// service is search/v1 on one app.
type service struct {
	app core.App

	mu       sync.Mutex
	versions map[string]int64     // bucket id -> bumped on every change to its chunks
	vecs     map[string]*vecCache // bucket id -> its vectors, as of a version
	queries  map[string][]float32 // model|dimensions|query -> its vector

	work        sync.Mutex // writes to a document's chunks, one at a time
	drainMu     sync.Mutex // one document is embedded at a time
	workerOnce  sync.Once
	kick        chan struct{}
	stop        chan struct{}
	stopOnce    sync.Once
	lastRequest time.Time // the last embedding request, for pacing (under drainMu)
}

func newService(app core.App) *service {
	return &service{app: app, versions: map[string]int64{}, vecs: map[string]*vecCache{}, queries: map[string][]float32{},
		kick: make(chan struct{}, 1), stop: make(chan struct{})}
}

func (s *service) bump(bucketID string) {
	s.mu.Lock()
	s.versions[bucketID]++
	s.mu.Unlock()
}

func badRequest(msg string) error {
	return &extkit.Error{Code: "bad_request", Status: 400, Message: msg}
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ---- buckets

func (s *service) findBucket(name string) (*core.Record, error) {
	if !nameRe.MatchString(name) {
		return nil, searchv1.ErrNoBucket
	}
	r, err := s.app.FindFirstRecordByData(BucketsCollection, "name", name)
	if err != nil {
		return nil, searchv1.ErrNoBucket
	}
	return r, nil
}

type bucketStats struct {
	Bucket    string `db:"bucket"`
	Documents int    `db:"documents"`
	Bytes     int64  `db:"bytes"`
	Chunks    int    `db:"chunks"`
	Pending   int    `db:"pending"`
	Errors    int    `db:"errors"`
}

func (s *service) stats(bucketID string) map[string]bucketStats {
	q := s.app.DB().Select("bucket", "count(*) AS documents", "coalesce(sum(size),0) AS bytes", "coalesce(sum(chunks),0) AS chunks",
		"coalesce(sum(status IN ('queued','embedding')),0) AS pending", "coalesce(sum(status = 'error'),0) AS errors").
		From(DocumentsCollection).GroupBy("bucket")
	if bucketID != "" {
		q = q.Where(dbx.HashExp{"bucket": bucketID})
	}
	var rows []bucketStats
	_ = q.All(&rows)
	out := map[string]bucketStats{}
	for _, r := range rows {
		out[r.Bucket] = r
	}
	return out
}

func bucketOf(r *core.Record, st bucketStats) searchv1.Bucket {
	return searchv1.Bucket{Name: r.GetString("name"), Title: r.GetString("title"), Description: r.GetString("description"),
		Owner: r.GetString("owner"), Model: r.GetString("model"), Dimensions: r.GetInt("dimensions"),
		MaxDocuments: r.GetInt("max_documents"), MaxBytes: int64(r.GetInt("max_bytes")),
		Documents: st.Documents, Chunks: st.Chunks, Bytes: st.Bytes, Pending: st.Pending, Errors: st.Errors,
		Created: r.GetDateTime("created").Time(), Updated: r.GetDateTime("updated").Time()}
}

// Buckets implements search/v1.
func (s *service) Buckets(_ context.Context, _ extkit.Auth) (searchv1.BucketList, error) {
	recs, err := s.app.FindRecordsByFilter(BucketsCollection, "", "name", 0, 0)
	if err != nil {
		return searchv1.BucketList{}, err
	}
	st := s.stats("")
	out := searchv1.BucketList{Buckets: []searchv1.Bucket{}}
	for _, r := range recs {
		out.Buckets = append(out.Buckets, bucketOf(r, st[r.Id]))
	}
	return out, nil
}

// Bucket implements search/v1.
func (s *service) Bucket(_ context.Context, _ extkit.Auth, in searchv1.Ref) (searchv1.Bucket, error) {
	r, err := s.findBucket(in.Name)
	if err != nil {
		return searchv1.Bucket{}, err
	}
	return bucketOf(r, s.stats(r.Id)[r.Id]), nil
}

// Ensure implements search/v1.
func (s *service) Ensure(ctx context.Context, a extkit.Auth, in searchv1.EnsureInput) (searchv1.Bucket, error) {
	if !nameRe.MatchString(in.Name) {
		return searchv1.Bucket{}, searchv1.ErrInvalidName
	}
	r, err := s.findBucket(in.Name)
	if err == nil {
		changed := false
		for field, v := range map[string]string{"title": in.Title, "description": in.Description, "owner": in.Owner} {
			if v != "" && r.GetString(field) == "" {
				r.Set(field, v)
				changed = true
			}
		}
		if changed {
			if err := s.app.Save(r); err != nil {
				return searchv1.Bucket{}, err
			}
		}
		return s.Bucket(ctx, a, searchv1.Ref{Name: in.Name})
	}
	col, err := s.app.FindCachedCollectionByNameOrId(BucketsCollection)
	if err != nil {
		return searchv1.Bucket{}, err
	}
	r = core.NewRecord(col)
	r.Set("name", in.Name)
	r.Set("title", in.Title)
	r.Set("description", in.Description)
	r.Set("owner", in.Owner)
	r.Set("model", in.Model)
	r.Set("dimensions", max(in.Dimensions, 0))
	r.Set("max_documents", DefaultMaxDocuments)
	r.Set("max_bytes", DefaultMaxBytes)
	if err := s.app.Save(r); err != nil {
		return searchv1.Bucket{}, err
	}
	return bucketOf(r, bucketStats{}), nil
}

// Update implements search/v1.
func (s *service) Update(ctx context.Context, a extkit.Auth, in searchv1.UpdateInput) (searchv1.Bucket, error) {
	r, err := s.findBucket(in.Name)
	if err != nil {
		return searchv1.Bucket{}, err
	}
	if in.Title != nil {
		r.Set("title", *in.Title)
	}
	if in.Description != nil {
		r.Set("description", *in.Description)
	}
	if in.MaxDocuments != nil {
		r.Set("max_documents", max(*in.MaxDocuments, 0))
	}
	if in.MaxBytes != nil {
		r.Set("max_bytes", max(*in.MaxBytes, 0))
	}
	reembed := false
	if in.Model != nil && strings.TrimSpace(*in.Model) != r.GetString("model") {
		r.Set("model", strings.TrimSpace(*in.Model))
		reembed = true
	}
	if in.Dimensions != nil && max(*in.Dimensions, 0) != r.GetInt("dimensions") {
		r.Set("dimensions", max(*in.Dimensions, 0))
		reembed = true
	}
	err = s.app.RunInTransaction(func(tx core.App) error {
		if err := tx.Save(r); err != nil {
			return err
		}
		if !reembed {
			return nil
		}
		// vectors from two models never mix: drop them all and queue every document that has chunks
		if _, err := tx.DB().Update(ChunksCollection, dbx.Params{"vector": ""}, dbx.HashExp{"bucket": r.Id}).Execute(); err != nil {
			return err
		}
		status := StatusQueued
		if r.GetString("model") == "" {
			status = StatusReady
		}
		_, err := tx.DB().Update(DocumentsCollection, dbx.Params{"status": status, "error": "", "cost": 0},
			dbx.And(dbx.HashExp{"bucket": r.Id}, dbx.NewExp("chunks > 0"))).Execute()
		return err
	})
	if err != nil {
		return searchv1.Bucket{}, err
	}
	if reembed {
		s.bump(r.Id)
		s.kickWorker()
	}
	return s.Bucket(ctx, a, searchv1.Ref{Name: in.Name})
}

// Delete implements search/v1.
func (s *service) Delete(_ context.Context, _ extkit.Auth, in searchv1.Ref) (searchv1.Removed, error) {
	r, err := s.findBucket(in.Name)
	if err != nil {
		return searchv1.Removed{}, err
	}
	docs, err := s.app.FindAllRecords(DocumentsCollection, dbx.HashExp{"bucket": r.Id})
	if err != nil {
		return searchv1.Removed{}, err
	}
	s.work.Lock()
	defer s.work.Unlock()
	err = s.app.RunInTransaction(func(tx core.App) error {
		if _, err := tx.DB().Delete(ChunksCollection, dbx.HashExp{"bucket": r.Id}).Execute(); err != nil {
			return err
		}
		if _, err := tx.DB().Delete(FTSTable, dbx.HashExp{"bucket": r.Id}).Execute(); err != nil {
			return err
		}
		for _, d := range docs {
			if err := tx.Delete(d); err != nil {
				return err
			}
		}
		return tx.Delete(r)
	})
	if err != nil {
		return searchv1.Removed{}, err
	}
	s.mu.Lock()
	delete(s.vecs, r.Id)
	delete(s.versions, r.Id)
	s.mu.Unlock()
	return searchv1.Removed{Count: 1}, nil
}

// ---- documents

func (s *service) findDocument(bucketID, name string) (*core.Record, error) {
	r, err := s.app.FindFirstRecordByFilter(DocumentsCollection, "bucket = {:b} && name = {:n}", dbx.Params{"b": bucketID, "n": name})
	if err != nil {
		return nil, searchv1.ErrNoDocument
	}
	return r, nil
}

func documentOf(bucket string, r *core.Record) searchv1.Document {
	return searchv1.Document{Bucket: bucket, Name: r.GetString("name"), Kind: r.GetString("kind"), Size: int64(r.GetInt("size")),
		Hash: r.GetString("hash"), Status: r.GetString("status"), Error: r.GetString("error"), Chunks: r.GetInt("chunks"),
		Tokens: r.GetInt("tokens"), CostMicroUSD: int64(r.GetInt("cost")),
		Created: r.GetDateTime("created").Time(), Updated: r.GetDateTime("updated").Time()}
}

// Add implements search/v1.
func (s *service) Add(_ context.Context, _ extkit.Auth, in searchv1.AddInput) (searchv1.Document, error) {
	return s.add(in.Bucket, in.Name, in.Text, false)
}

// Upload implements search/v1.
func (s *service) Upload(_ context.Context, _ extkit.Auth, in extkit.RawIn) (searchv1.Document, error) {
	if in.Body == nil {
		return searchv1.Document{}, badRequest("send the file as the request body")
	}
	b, err := io.ReadAll(io.LimitReader(in.Body, MaxFileBytes+1))
	if err != nil {
		return searchv1.Document{}, err
	}
	if len(b) > MaxFileBytes {
		return searchv1.Document{}, tooLarge(fmt.Sprintf("a document is at most %d MB", MaxFileBytes>>20))
	}
	if !isText(b) {
		return searchv1.Document{}, searchv1.ErrNotText
	}
	return s.add(in.Query.Get("bucket"), in.Query.Get("name"), string(b), false)
}

func tooLarge(msg string) error {
	return &extkit.Error{Code: searchv1.ErrTooLarge.Code, Status: searchv1.ErrTooLarge.Status, Message: msg}
}

// existingChunk is a stored chunk's vector, by what was embedded.
type existingChunk struct {
	Hash   string `db:"hash"`
	Vector string `db:"vector"`
}

// add splits a document and stores its chunks, reusing the vectors of chunks whose text didn't change; force splits
// it again even when its text is the same (reindex).
func (s *service) add(bucketName, name, text string, force bool) (searchv1.Document, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 1000 {
		return searchv1.Document{}, badRequest("give the document a name (at most 1000 characters)")
	}
	if len(text) > MaxFileBytes {
		return searchv1.Document{}, tooLarge(fmt.Sprintf("a document is at most %d MB", MaxFileBytes>>20))
	}
	if !isText([]byte(text)) {
		return searchv1.Document{}, searchv1.ErrNotText
	}
	b, err := s.findBucket(bucketName)
	if err != nil {
		return searchv1.Document{}, err
	}
	hash := sha(text)
	doc, _ := s.findDocument(b.Id, name)
	if doc != nil && !force && doc.GetString("hash") == hash && doc.GetString("status") != StatusError {
		d := documentOf(b.GetString("name"), doc)
		d.Unchanged = true
		return d, nil
	}

	// the bucket's limits, counting this document once
	st := s.stats(b.Id)[b.Id]
	docs, bytes := st.Documents, st.Bytes+int64(len(text))
	if doc == nil {
		docs++
	} else {
		bytes -= int64(doc.GetInt("size"))
	}
	if m := b.GetInt("max_documents"); m > 0 && docs > m {
		return searchv1.Document{}, tooLarge(fmt.Sprintf("the bucket holds at most %d documents", m))
	}
	if m := int64(b.GetInt("max_bytes")); m > 0 && bytes > m {
		return searchv1.Document{}, tooLarge(fmt.Sprintf("the bucket holds at most %d MB of text", m>>20))
	}

	kind := kindOf(name)
	pieces := split(kind, text)
	model := b.GetString("model")

	s.work.Lock() // not while the worker writes this document's vectors
	defer s.work.Unlock()

	reuse := map[string]string{}
	if doc != nil && model != "" {
		var old []existingChunk
		_ = s.app.DB().Select("hash", "vector").From(ChunksCollection).
			Where(dbx.And(dbx.HashExp{"document": doc.Id}, dbx.NewExp("vector != ''"))).All(&old)
		for _, c := range old {
			reuse[c.Hash] = c.Vector
		}
	}
	file, err := filesystem.NewFileFromBytes([]byte(text), sourceName(name))
	if err != nil {
		return searchv1.Document{}, err
	}

	err = s.app.RunInTransaction(func(tx core.App) error {
		if doc == nil {
			col, err := tx.FindCachedCollectionByNameOrId(DocumentsCollection)
			if err != nil {
				return err
			}
			doc = core.NewRecord(col)
			doc.Set("bucket", b.Id)
			doc.Set("name", name)
		} else {
			if _, err := tx.DB().Delete(ChunksCollection, dbx.HashExp{"document": doc.Id}).Execute(); err != nil {
				return err
			}
			if _, err := tx.DB().Delete(FTSTable, dbx.HashExp{"document": doc.Id}).Execute(); err != nil {
				return err
			}
		}
		total, missing := 0, 0
		for _, p := range pieces {
			total += tokens(p.Text)
			if model != "" && reuse[sha(embedText(name, p))] == "" {
				missing++
			}
		}
		status := StatusReady
		if missing > 0 {
			status = StatusQueued
		}
		doc.Set("kind", kind)
		doc.Set("size", len(text))
		doc.Set("hash", hash)
		doc.Set("status", status)
		doc.Set("error", "")
		doc.Set("chunks", len(pieces))
		doc.Set("tokens", total)
		if missing == len(pieces) {
			doc.Set("cost", 0)
		}
		doc.Set("source", file)
		if err := tx.Save(doc); err != nil {
			return err
		}
		for i, p := range pieces {
			id := security.PseudorandomString(15)
			h := sha(embedText(name, p))
			if _, err := tx.DB().Insert(ChunksCollection, dbx.Params{"id": id, "bucket": b.Id, "document": doc.Id, "seq": i,
				"heading": p.Heading, "start_line": p.Start, "end_line": p.End, "text": p.Text, "hash": h, "vector": reuse[h]}).Execute(); err != nil {
				return err
			}
			if _, err := tx.DB().Insert(FTSTable, dbx.Params{"text": p.Text, "heading": p.Heading, "chunk": id, "bucket": b.Id,
				"document": doc.Id, "name": name}).Execute(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return searchv1.Document{}, err
	}
	s.bump(b.Id)
	if doc.GetString("status") == StatusQueued {
		s.kickWorker()
	}
	return documentOf(b.GetString("name"), doc), nil
}

// sourceName is the stored file's name: the document's base name, with .txt when it has no extension.
func sourceName(name string) string {
	base := path.Base(strings.ReplaceAll(name, `\`, "/"))
	if path.Ext(base) == "" {
		base += ".txt"
	}
	return base
}

// Documents implements search/v1.
func (s *service) Documents(_ context.Context, _ extkit.Auth, in searchv1.DocumentsInput) (searchv1.DocumentList, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.DocumentList{}, err
	}
	filter, params := "bucket = {:b}", dbx.Params{"b": b.Id}
	if in.Prefix != "" {
		filter += " && name ~ {:p}"
		params["p"] = in.Prefix + "%"
	}
	recs, err := s.app.FindRecordsByFilter(DocumentsCollection, filter, "name", 0, 0, params)
	if err != nil {
		return searchv1.DocumentList{}, err
	}
	out := searchv1.DocumentList{Documents: []searchv1.Document{}}
	for _, r := range recs {
		out.Documents = append(out.Documents, documentOf(in.Bucket, r))
	}
	return out, nil
}

// Remove implements search/v1.
func (s *service) Remove(_ context.Context, _ extkit.Auth, in searchv1.RemoveInput) (searchv1.Removed, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.Removed{}, err
	}
	if len(in.Names) == 0 {
		return searchv1.Removed{}, badRequest("name the documents to remove")
	}
	s.work.Lock()
	defer s.work.Unlock()
	n := 0
	for _, name := range in.Names {
		doc, err := s.findDocument(b.Id, name)
		if err != nil {
			continue
		}
		err = s.app.RunInTransaction(func(tx core.App) error {
			if _, err := tx.DB().Delete(ChunksCollection, dbx.HashExp{"document": doc.Id}).Execute(); err != nil {
				return err
			}
			if _, err := tx.DB().Delete(FTSTable, dbx.HashExp{"document": doc.Id}).Execute(); err != nil {
				return err
			}
			return tx.Delete(doc)
		})
		if err != nil {
			return searchv1.Removed{Count: n}, err
		}
		n++
	}
	if n > 0 {
		s.bump(b.Id)
	}
	return searchv1.Removed{Count: n}, nil
}

// Reindex implements search/v1.
func (s *service) Reindex(_ context.Context, _ extkit.Auth, in searchv1.RemoveInput) (searchv1.Removed, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.Removed{}, err
	}
	var docs []*core.Record
	if len(in.Names) == 0 {
		docs, err = s.app.FindAllRecords(DocumentsCollection, dbx.HashExp{"bucket": b.Id})
		if err != nil {
			return searchv1.Removed{}, err
		}
	} else {
		for _, name := range in.Names {
			d, err := s.findDocument(b.Id, name)
			if err != nil {
				return searchv1.Removed{}, err
			}
			docs = append(docs, d)
		}
	}
	fsys, err := s.app.NewFilesystem()
	if err != nil {
		return searchv1.Removed{}, err
	}
	defer fsys.Close()
	n := 0
	for _, d := range docs {
		text, err := readSource(fsys, d)
		if err != nil {
			return searchv1.Removed{Count: n}, fmt.Errorf("%s: %w", d.GetString("name"), err)
		}
		if _, err := s.add(b.GetString("name"), d.GetString("name"), text, true); err != nil {
			return searchv1.Removed{Count: n}, fmt.Errorf("%s: %w", d.GetString("name"), err)
		}
		n++
	}
	return searchv1.Removed{Count: n}, nil
}

func readSource(fsys *filesystem.System, d *core.Record) (string, error) {
	file := d.GetString("source")
	if file == "" {
		return "", errors.New("its text wasn't kept; add it again")
	}
	r, err := fsys.GetReader(d.BaseFilesPath() + "/" + file)
	if err != nil {
		return "", err
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	return string(b), err
}

// of returns the service registered on app.
func of(app core.App) *service {
	s, _ := app.Store().Get(storeKey).(*service)
	return s
}
