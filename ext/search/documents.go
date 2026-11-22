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

func (s *service) findDocument(srcID, name string) (*core.Record, error) {
	r, err := s.app.FindFirstRecordByFilter(DocumentsCollection, "src = {:s} && name = {:n}", dbx.Params{"s": srcID, "n": name})
	if err != nil {
		return nil, errors.New("no document " + name)
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

func documentOf(bucket, source string, r *core.Record) searchv1.Document {
	return searchv1.Document{Bucket: bucket, Source: source, Name: r.GetString("name"), Kind: r.GetString("kind"),
		Size: int64(r.GetInt("size")), Hash: r.GetString("hash"), Status: r.GetString("status"), Error: r.GetString("error"),
		Chunks: r.GetInt("chunks"), Record: recordRef(r.GetString("record")), Tokens: r.GetInt("tokens"),
		CostMicroUSD: int64(r.GetInt("cost")), Created: r.GetDateTime("created").Time(), Updated: r.GetDateTime("updated").Time()}
}

func tooLarge(msg string) error {
	return &extkit.Error{Code: searchv1.ErrTooLarge.Code, Status: searchv1.ErrTooLarge.Status, Message: msg}
}

// docIn is a document a source brings: its name in the source, its text, how to split it and the record it's from.
type docIn struct {
	name, text, kind, record string
}

// addOpts: force splits again even when the text is the same.
type addOpts struct {
	force bool
}

// addDoc stores one document of a source: unchanged text does nothing; otherwise it's split, and every chunk whose
// content already has a vector from the app's model (anywhere in the app) reuses it.
func (s *service) addDoc(b, src *core.Record, in docIn, o addOpts) (*core.Record, bool, error) {
	name := strings.TrimSpace(in.name)
	if name == "" || len(name) > 1000 {
		return nil, false, badRequest("give the document a name (at most 1000 characters)")
	}
	if len(in.text) > MaxFileBytes {
		return nil, false, tooLarge(fmt.Sprintf("%s: a document is at most %d MB", name, MaxFileBytes>>20))
	}
	if !isText([]byte(in.text)) {
		return nil, false, searchv1.ErrNotText
	}
	hash := sha(in.text)
	doc, _ := s.findDocument(src.Id, name)
	if doc != nil && !o.force && doc.GetString("hash") == hash && doc.GetString("status") != StatusError {
		return doc, true, nil
	}

	// the bucket's limits, counting this document once
	st := s.stats(b.Id)[b.Id]
	docs, bytes := st.Documents, st.Bytes+int64(len(in.text))
	if doc == nil {
		docs++
	} else {
		bytes -= int64(doc.GetInt("size"))
	}
	if m := b.GetInt("max_documents"); m > 0 && docs > m {
		return nil, false, tooLarge(fmt.Sprintf("the bucket holds at most %d documents", m))
	}
	if m := int64(b.GetInt("max_bytes")); m > 0 && bytes > m {
		return nil, false, tooLarge(fmt.Sprintf("the bucket holds at most %d MB of text", m>>20))
	}

	kind := in.kind
	if kind == "" {
		kind = kindOf(name)
	}
	pieces := split(kind, in.text)
	hashes := make([]string, len(pieces))
	for i, p := range pieces {
		hashes[i] = sha(embedText(name, p))
	}
	model := s.knownModel()
	reuse := s.vectorsByHash(model, hashes)
	file, err := filesystem.NewFileFromBytes([]byte(in.text), sourceName(name))
	if err != nil {
		return nil, false, err
	}

	s.work.Lock() // not while the worker writes this document's vectors
	defer s.work.Unlock()
	err = s.app.RunInTransaction(func(tx core.App) error {
		if doc == nil {
			col, err := tx.FindCachedCollectionByNameOrId(DocumentsCollection)
			if err != nil {
				return err
			}
			doc = core.NewRecord(col)
			doc.Set("bucket", b.Id)
			doc.Set("src", src.Id)
			doc.Set("name", name)
		} else if err := deleteChunks(tx, dbx.HashExp{"document": doc.Id}); err != nil {
			return err
		}
		total, missing := 0, 0
		for i, p := range pieces {
			total += tokens(p.Text)
			if reuse[hashes[i]] == "" {
				missing++
			}
		}
		status := StatusReady
		if missing > 0 {
			status = StatusQueued
		}
		doc.Set("kind", kind)
		doc.Set("size", len(in.text))
		doc.Set("hash", hash)
		doc.Set("status", status)
		doc.Set("error", "")
		doc.Set("chunks", len(pieces))
		doc.Set("tokens", total)
		doc.Set("record", in.record)
		if missing == len(pieces) {
			doc.Set("cost", 0)
		}
		doc.Set("text", file)
		if err := tx.Save(doc); err != nil {
			return err
		}
		for i, p := range pieces {
			id := security.PseudorandomString(15)
			vec, m := reuse[hashes[i]], ""
			if vec != "" {
				m = model
			}
			if _, err := tx.DB().Insert(ChunksCollection, dbx.Params{"id": id, "bucket": b.Id, "document": doc.Id, "seq": i,
				"heading": p.Heading, "start_line": p.Start, "end_line": p.End, "text": p.Text, "hash": hashes[i],
				"vector": vec, "model": m}).Execute(); err != nil {
				return err
			}
			if _, err := tx.DB().Insert(FTSTable, dbx.Params{"text": p.Text, "heading": p.Heading, "chunk": id, "bucket": b.Id,
				"document": doc.Id, "name": name, "src": src.Id}).Execute(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	s.bump(b.Id)
	if doc.GetString("status") == StatusQueued {
		s.kickWorker()
	}
	return doc, false, nil
}

// vectorsByHash is the vectors from model of chunks anywhere in the app with these content hashes: identical chunks
// are embedded once, whichever source or bucket they're in.
func (s *service) vectorsByHash(model string, hashes []string) map[string]string {
	out := map[string]string{}
	if model == "" || len(hashes) == 0 {
		return out
	}
	for start := 0; start < len(hashes); start += 500 {
		batch := hashes[start:min(start+500, len(hashes))]
		args := make([]any, len(batch))
		for i, h := range batch {
			args[i] = h
		}
		var rows []struct {
			Hash   string `db:"hash"`
			Vector string `db:"vector"`
		}
		_ = s.app.DB().Select("hash", "vector").From(ChunksCollection).
			Where(dbx.And(dbx.In("hash", args...), dbx.HashExp{"model": model}, dbx.NewExp("vector != ''"))).
			GroupBy("hash").All(&rows)
		for _, r := range rows {
			out[r.Hash] = r.Vector
		}
	}
	return out
}

// deleteChunks removes chunks (and their keyword rows) matching where, a condition on the chunks table.
func deleteChunks(tx core.App, where dbx.Expression) error {
	var ids []string
	if err := tx.DB().Select("id").From(ChunksCollection).Where(where).Column(&ids); err != nil {
		return err
	}
	for start := 0; start < len(ids); start += 500 {
		batch := ids[start:min(start+500, len(ids))]
		args := make([]any, len(batch))
		for i, id := range batch {
			args[i] = id
		}
		if _, err := tx.DB().Delete(FTSTable, dbx.In("chunk", args...)).Execute(); err != nil {
			return err
		}
	}
	_, err := tx.DB().Delete(ChunksCollection, where).Execute()
	return err
}

// removeDoc deletes one document and its chunks.
func (s *service) removeDoc(bucketID string, doc *core.Record) error {
	s.work.Lock()
	defer s.work.Unlock()
	err := s.app.RunInTransaction(func(tx core.App) error {
		if err := deleteChunks(tx, dbx.HashExp{"document": doc.Id}); err != nil {
			return err
		}
		return tx.Delete(doc)
	})
	if err == nil {
		s.bump(bucketID)
	}
	return err
}

// sourceName is a stored text file's name: the document's base name, with .txt when it has no extension.
func sourceName(name string) string {
	base := path.Base(strings.ReplaceAll(name, `\`, "/"))
	if path.Ext(base) == "" || len(base) > 100 {
		base = "document.txt"
	}
	return base
}

// Documents implements search/v1.
func (s *service) Documents(_ context.Context, _ extkit.Auth, in searchv1.DocumentsInput) (searchv1.DocumentList, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.DocumentList{}, err
	}
	srcs, err := s.app.FindAllRecords(SourcesCollection, dbx.HashExp{"bucket": b.Id})
	if err != nil {
		return searchv1.DocumentList{}, err
	}
	names := map[string]string{}
	filter, params := "bucket = {:b}", dbx.Params{"b": b.Id}
	for _, src := range srcs {
		names[src.Id] = src.GetString("name")
		if in.Source != "" && src.GetString("name") == in.Source {
			filter += " && src = {:s}"
			params["s"] = src.Id
		}
	}
	if in.Source != "" && params["s"] == nil {
		return searchv1.DocumentList{}, searchv1.ErrNoSource
	}
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
		out.Documents = append(out.Documents, documentOf(in.Bucket, names[r.GetString("src")], r))
	}
	return out, nil
}

// readText reads a document's kept text.
func readText(fsys *filesystem.System, d *core.Record) (string, error) {
	file := d.GetString("text")
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
