package search

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/filesystem"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"

	searchv1 "pocketone.dev/ext/contracts/search/v1"
	"pocketone.dev/ext/extkit"
)

// Source statuses.
const (
	SourceReading = "reading"
	SourceReady   = "ready"
	SourceError   = "error"
)

// MaxZipBytes is the largest zip upload.
const MaxZipBytes = 100 << 20

// spec is a source's settings, stored as JSON.
type spec struct {
	URL        string   `json:"url,omitempty"`
	Every      string   `json:"every,omitempty"`
	Commits    int      `json:"commits,omitempty"`
	Exclude    []string `json:"exclude,omitempty"`
	Collection string   `json:"collection,omitempty"`
	Fields     []string `json:"fields,omitempty"`
	Filter     string   `json:"filter,omitempty"`
}

func specOf(r *core.Record) spec {
	var sp spec
	_ = json.Unmarshal([]byte(r.GetString("spec")), &sp)
	return sp
}

func setSpec(r *core.Record, sp spec) {
	b, _ := json.Marshal(sp)
	r.Set("spec", types.JSONRaw(b))
}

func (s *service) findSource(bucketID, name string) (*core.Record, error) {
	r, err := s.app.FindFirstRecordByFilter(SourcesCollection, "bucket = {:b} && name = {:n}", dbx.Params{"b": bucketID, "n": name})
	if err != nil {
		return nil, searchv1.ErrNoSource
	}
	return r, nil
}

type sourceStats struct {
	Src       string `db:"src"`
	Documents int    `db:"documents"`
	Bytes     int64  `db:"bytes"`
	Chunks    int    `db:"chunks"`
	Pending   int    `db:"pending"`
	Cost      int64  `db:"cost"`
}

func (s *service) sourceStats(bucketID string) map[string]sourceStats {
	var rows []sourceStats
	_ = s.app.DB().Select("src", "count(*) AS documents", "coalesce(sum(size),0) AS bytes", "coalesce(sum(chunks),0) AS chunks",
		"coalesce(sum(status IN ('queued','embedding')),0) AS pending", "coalesce(sum(cost),0) AS cost").
		From(DocumentsCollection).Where(dbx.HashExp{"bucket": bucketID}).GroupBy("src").All(&rows)
	out := map[string]sourceStats{}
	for _, r := range rows {
		out[r.Src] = r
	}
	return out
}

func timePtr(d types.DateTime) *time.Time {
	if d.IsZero() {
		return nil
	}
	t := d.Time()
	return &t
}

func sourceOf(bucket string, r *core.Record, st sourceStats) searchv1.Source {
	sp := specOf(r)
	return searchv1.Source{Bucket: bucket, Name: r.GetString("name"), Kind: r.GetString("kind"), URL: sp.URL, Every: sp.Every,
		Commits: sp.Commits, Exclude: sp.Exclude, Collection: sp.Collection, Fields: sp.Fields, Filter: sp.Filter,
		Hash: r.GetString("hash"), Status: r.GetString("status"), Error: r.GetString("error"), Documents: st.Documents,
		Chunks: st.Chunks, Bytes: st.Bytes, Pending: st.Pending, CostMicroUSD: st.Cost,
		Refreshed: timePtr(r.GetDateTime("refreshed")), Next: timePtr(r.GetDateTime("next")), Created: r.GetDateTime("created").Time()}
}

func (s *service) sourceResult(b, src *core.Record) (searchv1.Source, error) {
	fresh, err := s.app.FindRecordById(SourcesCollection, src.Id)
	if err != nil {
		return searchv1.Source{}, err
	}
	return sourceOf(b.GetString("name"), fresh, s.sourceStats(b.Id)[fresh.Id]), nil
}

// Sources implements search/v1.
func (s *service) Sources(_ context.Context, _ extkit.Auth, in searchv1.BucketRef) (searchv1.SourceList, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.SourceList{}, err
	}
	recs, err := s.app.FindRecordsByFilter(SourcesCollection, "bucket = {:b}", "name", 0, 0, dbx.Params{"b": b.Id})
	if err != nil {
		return searchv1.SourceList{}, err
	}
	st := s.sourceStats(b.Id)
	out := searchv1.SourceList{Sources: []searchv1.Source{}}
	for _, r := range recs {
		out.Sources = append(out.Sources, sourceOf(in.Bucket, r, st[r.Id]))
	}
	return out, nil
}

// putSource finds the source named name in the bucket (or replace, renamed to name) and sets its kind and spec,
// making it when it's missing. A source that changes kind loses its documents.
func (s *service) putSource(b *core.Record, name, replace, kind string, sp spec) (*core.Record, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 2000 {
		return nil, badRequest("give the source a name")
	}
	src, err := s.findSource(b.Id, name)
	if err != nil && replace != "" && replace != name {
		if old, err2 := s.findSource(b.Id, replace); err2 == nil {
			src, err = old, nil // a new version under a new name
		}
	}
	if err != nil {
		col, err := s.app.FindCachedCollectionByNameOrId(SourcesCollection)
		if err != nil {
			return nil, err
		}
		src = core.NewRecord(col)
		src.Set("bucket", b.Id)
	} else if src.GetString("kind") != kind {
		if err := s.clearSource(b.Id, src); err != nil {
			return nil, err
		}
	}
	src.Set("name", name)
	src.Set("kind", kind)
	setSpec(src, sp)
	if src.GetString("status") == "" {
		src.Set("status", SourceReady)
	}
	if err := s.app.Save(src); err != nil {
		return nil, err
	}
	return src, nil
}

// clearSource deletes a source's documents.
func (s *service) clearSource(bucketID string, src *core.Record) error {
	docs, err := s.app.FindAllRecords(DocumentsCollection, dbx.HashExp{"src": src.Id})
	if err != nil {
		return err
	}
	for _, d := range docs {
		if err := s.removeDoc(bucketID, d); err != nil {
			return err
		}
	}
	return nil
}

// syncDocs makes a source hold exactly docs: each added (unchanged ones cost nothing), the rest removed.
func (s *service) syncDocs(b, src *core.Record, docs []docIn, force bool) error {
	keep := map[string]bool{}
	var firstErr error
	for _, d := range docs {
		keep[strings.TrimSpace(d.name)] = true
		if _, _, err := s.addDoc(b, src, d, addOpts{force: force}); err != nil {
			if isLimit(err) {
				return err
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", d.name, err)
			}
		}
	}
	return errors2(firstErr, s.prune(b, src, keep))
}

func isLimit(err error) bool {
	e, ok := err.(*extkit.Error)
	return ok && e.Code == searchv1.ErrTooLarge.Code
}

func errors2(a, b error) error {
	if a != nil {
		return a
	}
	return b
}

// prune removes a source's documents that aren't in keep.
func (s *service) prune(b, src *core.Record, keep map[string]bool) error {
	docs, err := s.app.FindAllRecords(DocumentsCollection, dbx.HashExp{"src": src.Id})
	if err != nil {
		return err
	}
	for _, d := range docs {
		if !keep[d.GetString("name")] {
			if err := s.removeDoc(b.Id, d); err != nil {
				return err
			}
		}
	}
	return nil
}

// finish records how a read went: ready (or error), when, what was read, and when it's due again.
func (s *service) finish(src *core.Record, readErr error, state string) {
	fresh, err := s.app.FindRecordById(SourcesCollection, src.Id)
	if err != nil {
		return // removed meanwhile
	}
	if readErr != nil {
		fresh.Set("status", SourceError)
		msg := errText(readErr)
		if len(msg) > 2000 {
			msg = msg[:2000]
		}
		fresh.Set("error", msg)
	} else {
		fresh.Set("status", SourceReady)
		fresh.Set("error", "")
		if state != "" {
			fresh.Set("state", state)
		}
	}
	now := time.Now().UTC()
	fresh.Set("refreshed", now)
	if d := interval(specOf(fresh).Every); d > 0 {
		fresh.Set("next", now.Add(d))
	} else {
		fresh.Set("next", "")
	}
	if err := s.app.Save(fresh); err != nil {
		s.app.Logger().Warn("search: saving a source failed", "source", fresh.GetString("name"), "error", err)
	}
}

func interval(every string) time.Duration {
	switch every {
	case searchv1.EveryHour:
		return time.Hour
	case searchv1.EveryDay:
		return 24 * time.Hour
	case searchv1.EveryWeek:
		return 7 * 24 * time.Hour
	}
	return 0
}

// ---- files, text, folders, zips

// Add implements search/v1.
func (s *service) Add(_ context.Context, _ extkit.Auth, in searchv1.AddInput) (searchv1.Source, error) {
	kind := in.Kind
	if kind == "" {
		kind = searchv1.KindFile
	}
	if kind != searchv1.KindFile && kind != searchv1.KindText {
		return searchv1.Source{}, badRequest("kind is file or text")
	}
	return s.addFile(in.Bucket, in.Name, in.Replace, kind, in.Text)
}

func (s *service) addFile(bucket, name, replace, kind, text string) (searchv1.Source, error) {
	b, err := s.findBucket(bucket)
	if err != nil {
		return searchv1.Source{}, err
	}
	if !isText([]byte(text)) {
		return searchv1.Source{}, searchv1.ErrNotText
	}
	src, err := s.putSource(b, name, replace, kind, spec{})
	if err != nil {
		return searchv1.Source{}, err
	}
	src.Set("hash", sha(text))
	if err := s.app.Save(src); err != nil {
		return searchv1.Source{}, err
	}
	err = s.syncDocs(b, src, []docIn{{name: src.GetString("name"), text: text}}, false)
	s.finish(src, err, "")
	if err != nil {
		return searchv1.Source{}, err
	}
	return s.sourceResult(b, src)
}

// Upload implements search/v1.
func (s *service) Upload(_ context.Context, _ extkit.Auth, in extkit.RawIn) (searchv1.Source, error) {
	if in.Body == nil {
		return searchv1.Source{}, badRequest("send the file as the request body")
	}
	name := strings.TrimSpace(in.Query.Get("name"))
	isZip := strings.HasSuffix(strings.ToLower(name), ".zip")
	limit := int64(MaxFileBytes)
	if isZip {
		limit = MaxZipBytes
	}
	data, err := io.ReadAll(io.LimitReader(in.Body, limit+1))
	if err != nil {
		return searchv1.Source{}, err
	}
	if int64(len(data)) > limit {
		return searchv1.Source{}, tooLarge(fmt.Sprintf("%s is over %d MB", name, limit>>20))
	}
	if !isZip {
		return s.addFile(in.Query.Get("bucket"), name, in.Query.Get("replace"), searchv1.KindFile, string(data))
	}
	b, err := s.findBucket(in.Query.Get("bucket"))
	if err != nil {
		return searchv1.Source{}, err
	}
	docs, err := zipDocs(data)
	if err != nil {
		return searchv1.Source{}, badRequest(name + ": " + err.Error())
	}
	src, err := s.putSource(b, name, in.Query.Get("replace"), searchv1.KindZip, spec{})
	if err != nil {
		return searchv1.Source{}, err
	}
	file, err := filesystem.NewFileFromBytes(data, "source.zip")
	if err != nil {
		return searchv1.Source{}, err
	}
	src.Set("original", file)
	src.Set("hash", sha(string(data)))
	src.Set("status", SourceReading)
	if err := s.app.Save(src); err != nil {
		return searchv1.Source{}, err
	}
	err = s.syncDocs(b, src, docs, false)
	s.finish(src, err, "")
	if err != nil && isLimit(err) {
		return searchv1.Source{}, err
	}
	return s.sourceResult(b, src)
}

// zipDocs is the text files in a zip, by path (a single top folder they all share, as a repository download has,
// is dropped), skipping what multi-file sources always skip.
func zipDocs(data []byte) ([]docIn, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a zip file")
	}
	var names []string
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() {
			names = append(names, f.Name)
		}
	}
	top := commonTop(names)
	var docs []docIn
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		p := strings.TrimPrefix(f.Name, top)
		if skipPath(p, nil) || int64(f.UncompressedSize64) > MaxMultiFileBytes {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(rc, MaxMultiFileBytes+1))
		rc.Close()
		if err != nil || !isText(b) {
			continue
		}
		docs = append(docs, docIn{name: p, text: string(b)})
	}
	return docs, nil
}

// commonTop is "dir/" when every path starts with the same top folder.
func commonTop(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	first, _, ok := strings.Cut(paths[0], "/")
	if !ok {
		return ""
	}
	for _, p := range paths[1:] {
		if !strings.HasPrefix(p, first+"/") {
			return ""
		}
	}
	return first + "/"
}

// Put implements search/v1.
func (s *service) Put(_ context.Context, _ extkit.Auth, in searchv1.PutInput) (searchv1.Document, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.Document{}, err
	}
	src, err := s.findSource(b.Id, in.Source)
	if err != nil || src.GetString("kind") != searchv1.KindFolder {
		if src, err = s.putSource(b, in.Source, in.Replace, searchv1.KindFolder, spec{}); err != nil {
			return searchv1.Document{}, err
		}
	}
	if src.GetString("status") != SourceReading {
		src.Set("status", SourceReading)
		_ = s.app.Save(src)
	}
	doc, _, err := s.addDoc(b, src, docIn{name: in.Path, text: in.Text}, addOpts{})
	if err != nil {
		return searchv1.Document{}, err
	}
	return documentOf(b.GetString("name"), src.GetString("name"), doc), nil
}

// Prune implements search/v1.
func (s *service) Prune(_ context.Context, _ extkit.Auth, in searchv1.PruneInput) (searchv1.Source, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.Source{}, err
	}
	src, err := s.findSource(b.Id, in.Source)
	if err != nil {
		return searchv1.Source{}, err
	}
	keep := map[string]bool{}
	for _, p := range in.Keep {
		keep[strings.TrimSpace(p)] = true
	}
	err = s.prune(b, src, keep)
	s.finish(src, err, "")
	if err != nil {
		return searchv1.Source{}, err
	}
	return s.sourceResult(b, src)
}

// ---- URLs and collections

// gitHosts are the hosts whose owner/repo URLs are repositories.
var gitHosts = map[string]bool{"github.com": true, "gitlab.com": true, "codeberg.org": true}

// urlKind says whether a URL is a repository or a page.
func urlKind(u *url.URL) string {
	p := strings.Trim(u.Path, "/")
	if strings.HasSuffix(p, ".git") {
		return searchv1.KindGit
	}
	if gitHosts[strings.ToLower(u.Hostname())] && strings.Count(p, "/") == 1 {
		return searchv1.KindGit
	}
	return searchv1.KindPage
}

// AddURL implements search/v1.
func (s *service) AddURL(_ context.Context, _ extkit.Auth, in searchv1.URLInput) (searchv1.Source, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.Source{}, err
	}
	raw := strings.TrimSpace(in.URL)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return searchv1.Source{}, badRequest("give an http or https URL")
	}
	kind := in.Kind
	if kind == "" {
		kind = urlKind(u)
	}
	if kind != searchv1.KindPage && kind != searchv1.KindGit {
		return searchv1.Source{}, badRequest("kind is page or git")
	}
	if in.Every != "" && interval(in.Every) == 0 {
		return searchv1.Source{}, badRequest("every is hour, day or week")
	}
	sp := spec{URL: raw, Every: in.Every, Exclude: in.Exclude}
	if kind == searchv1.KindGit {
		sp.Commits = in.Commits
	}
	src, err := s.putSource(b, raw, "", kind, sp)
	if err != nil {
		return searchv1.Source{}, err
	}
	s.read(b, src, false)
	return s.sourceResult(b, src)
}

// Follow implements search/v1.
func (s *service) Follow(_ context.Context, _ extkit.Auth, in searchv1.FollowInput) (searchv1.Source, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.Source{}, err
	}
	sp := spec{Collection: in.Collection, Fields: in.Fields, Filter: in.Filter}
	if err := s.checkCollection(&sp); err != nil {
		return searchv1.Source{}, err
	}
	src, err := s.putSource(b, sp.Collection, "", searchv1.KindCollection, sp)
	if err != nil {
		return searchv1.Source{}, err
	}
	s.loadFollowers()
	s.read(b, src, false)
	return s.sourceResult(b, src)
}

// Refresh implements search/v1.
func (s *service) Refresh(_ context.Context, _ extkit.Auth, in searchv1.SourceRef) (searchv1.Source, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.Source{}, err
	}
	src, err := s.findSource(b.Id, in.Source)
	if err != nil {
		return searchv1.Source{}, err
	}
	s.read(b, src, true)
	return s.sourceResult(b, src)
}

// read reads a source (in the background on a server): fetches a page or repository, rereads a collection,
// resplits a zip or a file's kept text. force reads even what looks unchanged.
func (s *service) read(b, src *core.Record, force bool) {
	src.Set("status", SourceReading)
	src.Set("error", "")
	_ = s.app.Save(src)
	run := func() {
		state, err := s.readNow(b, src, force)
		s.finish(src, err, state)
	}
	if !background {
		run()
		return
	}
	go run()
}

// readNow does a source's reading and returns what it read (a page's ETag, a repository's commit).
func (s *service) readNow(b, src *core.Record, force bool) (string, error) {
	switch src.GetString("kind") {
	case searchv1.KindPage:
		return s.readPage(b, src, force)
	case searchv1.KindGit:
		return s.readGit(b, src, force)
	case searchv1.KindCollection:
		return "", s.syncCollection(b, src)
	case searchv1.KindZip:
		fsys, err := s.app.NewFilesystem()
		if err != nil {
			return "", err
		}
		defer fsys.Close()
		r, err := fsys.GetReader(src.BaseFilesPath() + "/" + src.GetString("original"))
		if err != nil {
			return "", fmt.Errorf("the zip wasn't kept; upload it again")
		}
		data, err := io.ReadAll(io.LimitReader(r, MaxZipBytes+1))
		r.Close()
		if err != nil {
			return "", err
		}
		docs, err := zipDocs(data)
		if err != nil {
			return "", err
		}
		return "", s.syncDocs(b, src, docs, force)
	default: // file, text, folder: split each document's kept text again
		fsys, err := s.app.NewFilesystem()
		if err != nil {
			return "", err
		}
		defer fsys.Close()
		docs, err := s.app.FindAllRecords(DocumentsCollection, dbx.HashExp{"src": src.Id})
		if err != nil {
			return "", err
		}
		for _, d := range docs {
			text, err := readText(fsys, d)
			if err != nil {
				return "", fmt.Errorf("%s: %w", d.GetString("name"), err)
			}
			if _, _, err := s.addDoc(b, src, docIn{name: d.GetString("name"), text: text, kind: d.GetString("kind"), record: d.GetString("record")}, addOpts{force: true}); err != nil {
				return "", err
			}
		}
		return "", nil
	}
}

// Remove implements search/v1.
func (s *service) Remove(_ context.Context, _ extkit.Auth, in searchv1.RemoveInput) (searchv1.Removed, error) {
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return searchv1.Removed{}, err
	}
	if len(in.Sources) == 0 {
		return searchv1.Removed{}, badRequest("name the sources to remove")
	}
	n := 0
	for _, name := range in.Sources {
		src, err := s.findSource(b.Id, name)
		if err != nil {
			continue
		}
		if err := s.clearSource(b.Id, src); err != nil {
			return searchv1.Removed{Count: n}, err
		}
		if err := s.app.Delete(src); err != nil {
			return searchv1.Removed{Count: n}, err
		}
		n++
	}
	s.loadFollowers()
	return searchv1.Removed{Count: n}, nil
}

// refreshDue reads every source whose interval has come round.
func (s *service) refreshDue() {
	due, err := s.app.FindRecordsByFilter(SourcesCollection, "next != '' && next <= {:now} && status != 'reading'", "next", 20, 0,
		dbx.Params{"now": types.NowDateTime().String()})
	if err != nil {
		return
	}
	for _, src := range due {
		b, err := s.app.FindRecordById(BucketsCollection, src.GetString("bucket"))
		if err != nil {
			continue
		}
		state, err := s.readNow(b, src, false)
		s.finish(src, err, state)
	}
}
