package search

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	searchv1 "pocketone.dev/ext/contracts/search/v1"
)

// follower is a bucket that follows a collection.
type follower struct {
	bucket string // the bucket's name
	src    searchv1.Source
}

// sourcePage is how many records a sync reads at a time.
const sourcePage = 200

// checkSource refuses a source whose collection or fields don't exist.
func (s *service) checkSource(src *searchv1.Source) error {
	col, err := s.app.FindCachedCollectionByNameOrId(src.Collection)
	if err != nil {
		return badRequest("source: no collection " + src.Collection)
	}
	if strings.HasPrefix(col.Name, "_") {
		return badRequest("source: a bucket can't follow a system collection")
	}
	if len(src.Fields) == 0 {
		return badRequest("source: name the fields to search (the first one is each record's title)")
	}
	for _, f := range src.Fields {
		if col.Fields.GetByName(f) == nil {
			return badRequest(fmt.Sprintf("source: %s has no field %q", col.Name, f))
		}
	}
	if src.Filter != "" {
		if _, err := s.app.FindRecordsByFilter(col, src.Filter, "", 1, 0); err != nil {
			return badRequest("source: the filter doesn't work: " + err.Error())
		}
	}
	src.Collection = col.Name
	return nil
}

// loadFollowers reads which buckets follow which collections.
func (s *service) loadFollowers() {
	recs, err := s.app.FindAllRecords(BucketsCollection)
	if err != nil {
		return // not migrated yet: try again on the next record change
	}
	m := map[string][]follower{}
	for _, r := range recs {
		if src := sourceOf(r); src != nil {
			m[src.Collection] = append(m[src.Collection], follower{bucket: r.GetString("name"), src: *src})
		}
	}
	s.mu.Lock()
	s.followers, s.followersLoaded = m, true
	s.mu.Unlock()
}

func (s *service) followersOf(collection string) []follower {
	if strings.HasPrefix(collection, "_") { // PocketBase's and extensions' own collections, search's included
		return nil
	}
	s.mu.Lock()
	loaded := s.followersLoaded
	s.mu.Unlock()
	if !loaded {
		s.loadFollowers()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.followers[collection]
}

// followChanged picks up a bucket's new source: it reads the collection's records (in the background on a server).
func (s *service) followChanged(b *core.Record) {
	s.loadFollowers()
	if sourceOf(b) == nil {
		return
	}
	run := func() {
		if _, err := s.syncSource(b); err != nil {
			s.app.Logger().Warn("search: reading a followed collection failed", "bucket", b.GetString("name"), "error", err)
		}
	}
	if !background {
		run()
		return
	}
	go run()
}

// bindSources keeps followed collections' records in their buckets as they're saved and deleted.
func (s *service) bindSources() {
	changed := func(e *core.RecordEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		for _, f := range s.followersOf(e.Record.Collection().Name) {
			if err := s.syncRecord(f, e.Record); err != nil {
				s.app.Logger().Warn("search: keeping a record in its bucket failed", "bucket", f.bucket, "record", e.Record.Id, "error", err)
			}
		}
		return nil
	}
	s.app.OnRecordAfterCreateSuccess().BindFunc(changed)
	s.app.OnRecordAfterUpdateSuccess().BindFunc(changed)
	s.app.OnRecordAfterDeleteSuccess().BindFunc(func(e *core.RecordEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		for _, f := range s.followersOf(e.Record.Collection().Name) {
			if b, err := s.findBucket(f.bucket); err == nil {
				_, _ = s.removeDocument(b, recordName(e.Record))
			}
		}
		return nil
	})
}

func recordName(r *core.Record) string { return r.Collection().Name + "/" + r.Id }

// syncRecord adds a record to a follower's bucket, or removes it when the filter no longer keeps it.
func (s *service) syncRecord(f follower, r *core.Record) error {
	b, err := s.findBucket(f.bucket)
	if err != nil {
		return nil
	}
	if f.src.Filter != "" {
		_, err := s.app.FindFirstRecordByFilter(r.Collection(), "id = {:id} && ("+f.src.Filter+")", dbx.Params{"id": r.Id})
		if err != nil {
			_, err := s.removeDocument(b, recordName(r))
			return err
		}
	}
	text := renderRecord(r, f.src.Fields)
	if strings.TrimSpace(text) == "" {
		_, err := s.removeDocument(b, recordName(r))
		return err
	}
	_, err = s.add(f.bucket, recordName(r), text, addOpts{kind: KindMarkdown, record: recordName(r)})
	return err
}

// syncSource reads every record the bucket's source keeps into it and removes documents of records it no longer keeps.
// It returns how many records it read.
func (s *service) syncSource(b *core.Record) (int, error) {
	src := sourceOf(b)
	if src == nil {
		return 0, nil
	}
	col, err := s.app.FindCachedCollectionByNameOrId(src.Collection)
	if err != nil {
		return 0, err
	}
	f := follower{bucket: b.GetString("name"), src: *src}
	seen := map[string]bool{}
	n := 0
	for offset := 0; ; offset += sourcePage {
		recs, err := s.app.FindRecordsByFilter(col, src.Filter, "id", sourcePage, offset)
		if err != nil {
			return n, err
		}
		for _, r := range recs {
			seen[recordName(r)] = true
			text := renderRecord(r, src.Fields)
			if strings.TrimSpace(text) == "" {
				continue
			}
			if _, err := s.add(f.bucket, recordName(r), text, addOpts{kind: KindMarkdown, record: recordName(r)}); err != nil {
				return n, fmt.Errorf("%s: %w", recordName(r), err)
			}
			n++
		}
		if len(recs) < sourcePage {
			break
		}
	}
	// records the source no longer keeps (deleted, filtered out, another collection before)
	docs, err := s.app.FindAllRecords(DocumentsCollection, dbx.HashExp{"bucket": b.Id}, dbx.NewExp("record != ''"))
	if err != nil {
		return n, err
	}
	for _, d := range docs {
		if !seen[d.GetString("record")] {
			if _, err := s.removeDocument(b, d.GetString("name")); err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

var tagRe = regexp.MustCompile(`(?s)<[^>]*>`)
var blockTagRe = regexp.MustCompile(`(?i)</?(p|div|h[1-6]|li|ul|ol|br|tr|blockquote|pre)[^>]*>`)

// renderRecord is a record as Markdown: the first field as its title, the others below it (editor fields' HTML as
// text).
func renderRecord(r *core.Record, fields []string) string {
	var parts []string
	for i, name := range fields {
		f := r.Collection().Fields.GetByName(name)
		if f == nil {
			continue
		}
		v := strings.TrimSpace(r.GetString(name))
		if f.Type() == core.FieldTypeEditor {
			v = blockTagRe.ReplaceAllString(v, "\n\n")
			v = strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(v, "")))
		}
		if v == "" {
			continue
		}
		if i == 0 {
			v = "# " + strings.ReplaceAll(v, "\n", " ")
		}
		parts = append(parts, v)
	}
	return strings.Join(parts, "\n\n")
}
