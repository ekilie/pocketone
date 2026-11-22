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

// follower is a collection source.
type follower struct {
	bucket string // the bucket's id
	source string // the source's id
	sp     spec
}

// collectionPage is how many records a collection read takes at a time.
const collectionPage = 200

// checkCollection refuses a collection source whose collection or fields don't exist, and names the collection by
// its name.
func (s *service) checkCollection(sp *spec) error {
	col, err := s.app.FindCachedCollectionByNameOrId(sp.Collection)
	if err != nil {
		return badRequest("no collection " + sp.Collection)
	}
	if strings.HasPrefix(col.Name, "_") {
		return badRequest("a bucket can't follow a system collection")
	}
	if len(sp.Fields) == 0 {
		return badRequest("name the fields to search (the first one is each record's title)")
	}
	for _, f := range sp.Fields {
		if col.Fields.GetByName(f) == nil {
			return badRequest(fmt.Sprintf("%s has no field %q", col.Name, f))
		}
	}
	if sp.Filter != "" {
		if _, err := s.app.FindRecordsByFilter(col, sp.Filter, "", 1, 0); err != nil {
			return badRequest("the filter doesn't work: " + err.Error())
		}
	}
	sp.Collection = col.Name
	return nil
}

// loadFollowers reads which sources follow which collections.
func (s *service) loadFollowers() {
	recs, err := s.app.FindAllRecords(SourcesCollection, dbx.HashExp{"kind": searchv1.KindCollection})
	if err != nil {
		return // not migrated yet: try again on the next record change
	}
	m := map[string][]follower{}
	for _, r := range recs {
		sp := specOf(r)
		m[sp.Collection] = append(m[sp.Collection], follower{bucket: r.GetString("bucket"), source: r.Id, sp: sp})
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

// bindCollections keeps followed collections' records in their sources as they're saved and deleted.
func (s *service) bindCollections() {
	changed := func(e *core.RecordEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		for _, f := range s.followersOf(e.Record.Collection().Name) {
			if err := s.syncRecord(f, e.Record, false); err != nil {
				s.app.Logger().Warn("search: keeping a record in its bucket failed", "record", e.Record.Id, "error", err)
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
			if err := s.syncRecord(f, e.Record, true); err != nil {
				s.app.Logger().Warn("search: removing a record from its bucket failed", "record", e.Record.Id, "error", err)
			}
		}
		return nil
	})
}

func recordName(r *core.Record) string { return r.Collection().Name + "/" + r.Id }

// syncRecord adds a record to a collection source, or removes it (deleted, or the filter no longer keeps it).
func (s *service) syncRecord(f follower, r *core.Record, deleted bool) error {
	b, err := s.app.FindRecordById(BucketsCollection, f.bucket)
	if err != nil {
		return nil
	}
	src, err := s.app.FindRecordById(SourcesCollection, f.source)
	if err != nil {
		return nil
	}
	keep := !deleted
	if keep && f.sp.Filter != "" {
		_, err := s.app.FindFirstRecordByFilter(r.Collection(), "id = {:id} && ("+f.sp.Filter+")", dbx.Params{"id": r.Id})
		keep = err == nil
	}
	text := ""
	if keep {
		text = renderRecord(r, f.sp.Fields)
	}
	if strings.TrimSpace(text) == "" {
		if doc, err := s.findDocument(src.Id, recordName(r)); err == nil {
			return s.removeDoc(b.Id, doc)
		}
		return nil
	}
	_, _, err = s.addDoc(b, src, docIn{name: recordName(r), text: text, kind: KindMarkdown, record: recordName(r)}, addOpts{})
	return err
}

// syncCollection reads every record a collection source keeps into it, and removes the documents of records it no
// longer keeps.
func (s *service) syncCollection(b, src *core.Record) error {
	sp := specOf(src)
	col, err := s.app.FindCachedCollectionByNameOrId(sp.Collection)
	if err != nil {
		return err
	}
	var docs []docIn
	for offset := 0; ; offset += collectionPage {
		recs, err := s.app.FindRecordsByFilter(col, sp.Filter, "id", collectionPage, offset)
		if err != nil {
			return err
		}
		for _, r := range recs {
			if text := renderRecord(r, sp.Fields); strings.TrimSpace(text) != "" {
				docs = append(docs, docIn{name: recordName(r), text: text, kind: KindMarkdown, record: recordName(r)})
			}
		}
		if len(recs) < collectionPage {
			break
		}
	}
	return s.syncDocs(b, src, docs, false)
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
