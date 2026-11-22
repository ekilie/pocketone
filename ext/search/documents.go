package search

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/filesystem"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/security"

	searchv1 "pocketone.dev/ext/contracts/search/v1"
	"pocketone.dev/ext/extkit"
)

func (s *service) findDocument(bucketID, name string) (*core.Record, error) {
	r, err := s.app.FindFirstRecordByFilter(DocumentsCollection, "bucket = {:b} && name = {:n}", dbx.Params{"b": bucketID, "n": name})
	if err != nil {
		return nil, searchv1.ErrNoDocument
	}
	return r, nil
}

// recordRef parses a document's "collection/id".
func recordRef(s string) *searchv1.RecordRef {
	c, id, ok := strings.Cut(s, "/")
	if !ok || c == "" || id == "" {
		return nil
	}
	return &searchv1.RecordRef{Collection: c, ID: id}
}

func documentOf(bucket string, r *core.Record) searchv1.Document {
	return searchv1.Document{Bucket: bucket, Name: r.GetString("name"), Kind: r.GetString("kind"), Size: int64(r.GetInt("size")),
		Hash: r.GetString("hash"), Status: r.GetString("status"), Error: r.GetString("error"), Chunks: r.GetInt("chunks"),
		Record: recordRef(r.GetString("record")), Tokens: r.GetInt("tokens"), CostMicroUSD: int64(r.GetInt("cost")),
		Created: r.GetDateTime("created").Time(), Updated: r.GetDateTime("updated").Time()}
}

// Add implements search/v1.
func (s *service) Add(_ context.Context, _ extkit.Auth, in searchv1.AddInput) (searchv1.Document, error) {
	return s.add(in.Bucket, in.Name, in.Text, addOpts{})
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
	return s.add(in.Query.Get("bucket"), in.Query.Get("name"), string(b), addOpts{})
}

func tooLarge(msg string) error {
	return &extkit.Error{Code: searchv1.ErrTooLarge.Code, Status: searchv1.ErrTooLarge.Status, Message: msg}
}

// existingChunk is a stored chunk's vector, by what was embedded.
type existingChunk struct {
	Hash   string `db:"hash"`
	Vector string `db:"vector"`
	Model  string `db:"model"`
}

// addOpts: force splits again even when the text is the same (reindex); kind overrides the name's; record is the
// "collection/id" a followed record's document was made from.
type addOpts struct {
	force  bool
	kind   string
	record string
}

// add splits a document and stores its chunks, reusing the vectors of chunks whose text didn't change.
func (s *service) add(bucketName, name, text string, o addOpts) (searchv1.Document, error) {
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
	if doc != nil && !o.force && doc.GetString("hash") == hash && doc.GetString("status") != StatusError {
		d := documentOf(b.GetString("name"), doc)
		d.Unchanged = true
		return d, nil
	}

	// the bucket's limits, counting this document once (no document limit for a followed collection)
	st := s.stats(b.Id)[b.Id]
	docs, bytes := st.Documents, st.Bytes+int64(len(text))
	if doc == nil {
		docs++
	} else {
		bytes -= int64(doc.GetInt("size"))
	}
	if m := b.GetInt("max_documents"); m > 0 && docs > m && sourceOf(b) == nil {
		return searchv1.Document{}, tooLarge(fmt.Sprintf("the bucket holds at most %d documents", m))
	}
	if m := int64(b.GetInt("max_bytes")); m > 0 && bytes > m {
		return searchv1.Document{}, tooLarge(fmt.Sprintf("the bucket holds at most %d MB of text", m>>20))
	}

	kind := o.kind
	if kind == "" {
		kind = kindOf(name)
	}
	pieces := split(kind, text)
	model := s.knownModel()

	s.work.Lock() // not while the worker writes this document's vectors
	defer s.work.Unlock()

	reuse := map[string]existingChunk{}
	if doc != nil {
		var old []existingChunk
		_ = s.app.DB().Select("hash", "vector", "model").From(ChunksCollection).
			Where(dbx.And(dbx.HashExp{"document": doc.Id}, dbx.NewExp("vector != ''"))).All(&old)
		for _, c := range old {
			reuse[c.Hash] = c
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
			if c, ok := reuse[sha(embedText(name, p))]; !ok || model == "" || c.Model != model {
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
		doc.Set("record", o.record)
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
			c := reuse[h]
			if _, err := tx.DB().Insert(ChunksCollection, dbx.Params{"id": id, "bucket": b.Id, "document": doc.Id, "seq": i,
				"heading": p.Heading, "start_line": p.Start, "end_line": p.End, "text": p.Text, "hash": h,
				"vector": c.Vector, "model": c.Model}).Execute(); err != nil {
				return err
			}
			if _, err := tx.DB().Insert(FTSTable, dbx.Params{"text": p.Text, "heading": p.Heading, "chunk": id, "bucket": b.Id,
				"document": doc.Id, "name": name, "record": o.record}).Execute(); err != nil {
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
	n := 0
	for _, name := range in.Names {
		ok, err := s.removeDocument(b, name)
		if err != nil {
			return searchv1.Removed{Count: n}, err
		}
		if ok {
			n++
		}
	}
	return searchv1.Removed{Count: n}, nil
}

// removeDocument deletes one document and its chunks; false when there's none by that name.
func (s *service) removeDocument(b *core.Record, name string) (bool, error) {
	doc, err := s.findDocument(b.Id, name)
	if err != nil {
		return false, nil
	}
	s.work.Lock()
	defer s.work.Unlock()
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
		return false, err
	}
	s.bump(b.Id)
	return true, nil
}

// Reindex implements search/v1.
func (s *service) Reindex(_ context.Context, _ extkit.Auth, in searchv1.RemoveInput) (searchv1.Removed, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.Removed{}, err
	}
	if len(in.Names) == 0 && sourceOf(b) != nil {
		n, err := s.syncSource(b)
		return searchv1.Removed{Count: n}, err
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
		o := addOpts{force: true, kind: d.GetString("kind"), record: d.GetString("record")}
		if _, err := s.add(b.GetString("name"), d.GetString("name"), text, o); err != nil {
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
