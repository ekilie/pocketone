// Package search provides search/v1: named buckets of content an app searches. An app makes the bucket it needs
// ($search.ensure) and searches it ($search.searchAsync), or asks a model call to enrich its prompt from it
// (`enrich` on $llm calls); its admins load files and text into it from the dashboard's Search screen, or the bucket
// follows one of the app's collections. In page code it's $search.
//
// A bucket is a record in _search_buckets; each document (a file, pasted text or a record, by name) is in
// _search_documents with its text kept as a file; its chunks are in _search_chunks, with their heading path, lines,
// embedding and the model that made it, and in the FTS5 table _search_fts. Adding a document splits it at once; a
// background worker embeds the chunks with the app's one embedding model (llm.embed/v1, picked automatically and kept
// in kv). Search embeds the query once and ranks the chunks of every bucket asked for by cosine similarity (a loop over
// vectors cached in memory) and BM25 together, merged by reciprocal rank.
package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"sync"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	searchv1 "pocketone.dev/ext/contracts/search/v1"
	"pocketone.dev/ext/extkit"
)

// Name is the extension's key.
const Name = "search"

// Limits a new bucket starts with; its admins can change them. A bucket that follows a collection has no document
// limit (one document per record).
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

// Contracts implements extkit.Declarer: it embeds through llm.embed/v1 and keeps its model in kv/v1.
func (*Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("search/v1"), WorksWith: extkit.IDs("llm.embed/v1", "kv/v1")}
}

// Register implements extkit.Extension.
func (*Extension) Register(app core.App, _ extkit.Config) error {
	s := newService(app)
	app.Store().Set(storeKey, s)
	searchv1.Contract.Provide(app, s)
	s.bindCollections()
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		s.loadFollowers()
		s.kickWorker()  // resume what was queued before a restart
		s.resumeReads() // and sources a restart cut off mid-read
		// URL and repository sources with an interval, read when they're due
		app.Cron().MustAdd("__pocketoneSearchRefresh__", "*/5 * * * *", func() { s.refreshDue() })
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

	mu              sync.Mutex
	versions        map[string]int64      // bucket id -> bumped on every change to its chunks
	vecs            map[string]*vecCache  // bucket id -> its vectors, as of a version
	queries         map[string][]float32  // model|query -> its vector
	model           string                // the app's embedding model, once known
	followers       map[string][]follower // collection name -> the buckets that follow it
	followersLoaded bool

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
		followers: map[string][]follower{}, kick: make(chan struct{}, 1), stop: make(chan struct{})}
}

func (s *service) bump(bucketID string) {
	s.mu.Lock()
	s.versions[bucketID]++
	s.mu.Unlock()
}

func (s *service) bumpAll() {
	s.mu.Lock()
	for id := range s.versions {
		s.versions[id]++
	}
	clear(s.vecs)
	clear(s.queries)
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
	Sources   int    `db:"-"`
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
	// sources, and the ones whose last read failed
	var srcs []struct {
		Bucket string `db:"bucket"`
		N      int    `db:"n"`
		Errors int    `db:"errors"`
	}
	sq := s.app.DB().Select("bucket", "count(*) AS n", "coalesce(sum(status = 'error'),0) AS errors").From(SourcesCollection).GroupBy("bucket")
	if bucketID != "" {
		sq = sq.Where(dbx.HashExp{"bucket": bucketID})
	}
	_ = sq.All(&srcs)
	for _, r := range srcs {
		st := out[r.Bucket]
		st.Bucket, st.Sources, st.Errors = r.Bucket, r.N, st.Errors+r.Errors
		out[r.Bucket] = st
	}
	return out
}

func bucketOf(r *core.Record, st bucketStats) searchv1.Bucket {
	return searchv1.Bucket{Name: r.GetString("name"), Title: r.GetString("title"), Description: r.GetString("description"),
		Owner:        r.GetString("owner"),
		MaxDocuments: r.GetInt("max_documents"), MaxBytes: int64(r.GetInt("max_bytes")),
		Sources: st.Sources, Documents: st.Documents, Chunks: st.Chunks, Bytes: st.Bytes, Pending: st.Pending, Errors: st.Errors,
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
	r.Set("max_documents", DefaultMaxDocuments)
	r.Set("max_bytes", DefaultMaxBytes)
	if err := s.app.Save(r); err != nil {
		return searchv1.Bucket{}, err
	}
	return s.Bucket(ctx, a, searchv1.Ref{Name: in.Name})
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
	if err := s.app.Save(r); err != nil {
		return searchv1.Bucket{}, err
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
	err = s.tx(func(tx core.App) error {
		if _, err := tx.DB().Delete(ChunksCollection, dbx.HashExp{"bucket": r.Id}).Execute(); err != nil {
			return err
		}
		if _, err := tx.DB().Delete(FTSTable, dbx.HashExp{"bucket": r.Id}).Execute(); err != nil {
			return err
		}
		srcs, err := tx.FindAllRecords(SourcesCollection, dbx.HashExp{"bucket": r.Id})
		if err != nil {
			return err
		}
		for _, d := range docs {
			if err := tx.Delete(d); err != nil {
				return err
			}
		}
		for _, src := range srcs {
			if err := tx.Delete(src); err != nil {
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
	s.loadFollowers()
	return searchv1.Removed{Count: 1}, nil
}

// of returns the service registered on app.
func of(app core.App) *service {
	s, _ := app.Store().Get(storeKey).(*service)
	return s
}

// tx runs fn in a transaction, again (after a short wait) when SQLite says the database is locked: a transaction that
// read before it wrote loses to another connection's commit (SQLITE_BUSY_SNAPSHOT), and PocketBase retries single
// statements but not whole transactions.
func (s *service) tx(fn func(tx core.App) error) error {
	var err error
	for attempt, wait := 0, 50*time.Millisecond; attempt < 8; attempt, wait = attempt+1, wait*2 {
		if err = s.app.RunInTransaction(fn); err == nil || !isLocked(err) {
			return err
		}
		time.Sleep(wait)
	}
	return err
}

func isLocked(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "table is locked")
}
