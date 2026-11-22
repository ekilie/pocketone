package search

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"pocketone.dev/ext/extkit"
)

// The collections, named `_<extension>_<table>`: system collections with no API rules, so only superusers and Go/JS
// code reach them. The keyword index is an SQLite FTS5 table beside them.
const (
	BucketsCollection     = "_search_buckets"
	BucketsCollectionID   = "pbc_search_buckets"
	DocumentsCollection   = "_search_documents"
	DocumentsCollectionID = "pbc_search_documents"
	ChunksCollection      = "_search_chunks"
	ChunksCollectionID    = "pbc_search_chunks"
	FTSTable              = "_search_fts"
)

// MaxFileBytes is the largest document.
const MaxFileBytes = 10 << 20

// Migrations implements extkit.Migrator: 0001 creates the buckets, documents, chunks and the keyword index.
func (*Extension) Migrations() []extkit.Migration {
	return []extkit.Migration{{Name: "0001_buckets", Up: createTables, Down: dropTables}}
}

func createTables(tx core.App) error {
	buckets := core.NewBaseCollection(BucketsCollection, BucketsCollectionID)
	buckets.System = true
	buckets.Fields.Add(
		&core.TextField{Id: "sb_name", Name: "name", Required: true, Max: 64, Pattern: `^[A-Za-z0-9][A-Za-z0-9_-]*$`},
		&core.TextField{Id: "sb_title", Name: "title", Max: 200},
		&core.TextField{Id: "sb_description", Name: "description", Max: 2000, Help: "What belongs in the bucket, shown to the admins who load it."},
		&core.TextField{Id: "sb_owner", Name: "owner", Max: 200, Help: "Who made it (the app's name)."},
		&core.TextField{Id: "sb_model", Name: "model", Max: 300, Help: "The embedding model (provider/model); empty searches by keywords only."},
		&core.NumberField{Id: "sb_dimensions", Name: "dimensions", OnlyInt: true},
		&core.NumberField{Id: "sb_max_documents", Name: "max_documents", OnlyInt: true},
		&core.NumberField{Id: "sb_max_bytes", Name: "max_bytes", OnlyInt: true},
		&core.AutodateField{Id: "sb_created", Name: "created", OnCreate: true, System: true},
		&core.AutodateField{Id: "sb_updated", Name: "updated", OnCreate: true, OnUpdate: true, System: true},
	)
	buckets.AddIndex("idx_search_buckets_name", true, "name", "")
	if err := tx.Save(buckets); err != nil {
		return err
	}

	docs := core.NewBaseCollection(DocumentsCollection, DocumentsCollectionID)
	docs.System = true
	docs.Fields.Add(
		&core.RelationField{Id: "sd_bucket", Name: "bucket", CollectionId: BucketsCollectionID, MaxSelect: 1, Required: true, CascadeDelete: true},
		&core.TextField{Id: "sd_name", Name: "name", Required: true, Max: 1000},
		&core.SelectField{Id: "sd_kind", Name: "kind", MaxSelect: 1, Values: []string{KindMarkdown, KindCode, KindText}},
		&core.NumberField{Id: "sd_size", Name: "size", OnlyInt: true},
		&core.TextField{Id: "sd_hash", Name: "hash", Max: 64},
		&core.SelectField{Id: "sd_status", Name: "status", MaxSelect: 1, Values: []string{StatusQueued, StatusEmbedding, StatusReady, StatusError}},
		&core.TextField{Id: "sd_error", Name: "error", Max: 2000},
		&core.NumberField{Id: "sd_chunks", Name: "chunks", OnlyInt: true},
		&core.NumberField{Id: "sd_tokens", Name: "tokens", OnlyInt: true},
		&core.NumberField{Id: "sd_cost", Name: "cost", OnlyInt: true, Help: "What embedding it has cost, in millionths of a US dollar."},
		&core.FileField{Id: "sd_source", Name: "source", MaxSelect: 1, MaxSize: MaxFileBytes, Protected: true, Help: "The text as added, to split it again later."},
		&core.AutodateField{Id: "sd_created", Name: "created", OnCreate: true, System: true},
		&core.AutodateField{Id: "sd_updated", Name: "updated", OnCreate: true, OnUpdate: true, System: true},
	)
	docs.AddIndex("idx_search_documents_name", true, "bucket, name", "")
	docs.AddIndex("idx_search_documents_status", false, "status", "")
	if err := tx.Save(docs); err != nil {
		return err
	}

	chunks := core.NewBaseCollection(ChunksCollection, ChunksCollectionID)
	chunks.System = true
	chunks.Fields.Add(
		&core.RelationField{Id: "sc_bucket", Name: "bucket", CollectionId: BucketsCollectionID, MaxSelect: 1, Required: true, CascadeDelete: true},
		&core.RelationField{Id: "sc_document", Name: "document", CollectionId: DocumentsCollectionID, MaxSelect: 1, Required: true, CascadeDelete: true},
		&core.NumberField{Id: "sc_seq", Name: "seq", OnlyInt: true},
		&core.TextField{Id: "sc_heading", Name: "heading", Max: 1000},
		&core.NumberField{Id: "sc_start_line", Name: "start_line", OnlyInt: true},
		&core.NumberField{Id: "sc_end_line", Name: "end_line", OnlyInt: true},
		&core.TextField{Id: "sc_text", Name: "text", Max: 1 << 16},
		&core.TextField{Id: "sc_hash", Name: "hash", Max: 64, Help: "sha256 of what was embedded, to reuse its vector when the document changes."},
		&core.TextField{Id: "sc_vector", Name: "vector", Max: 1 << 20, Help: "The embedding: little-endian float32s, base64."},
	)
	chunks.AddIndex("idx_search_chunks_document", true, "document, seq", "")
	chunks.AddIndex("idx_search_chunks_bucket", false, "bucket", "")
	if err := tx.Save(chunks); err != nil {
		return err
	}

	_, err := tx.DB().NewQuery(`CREATE VIRTUAL TABLE IF NOT EXISTS ` + FTSTable + ` USING fts5(
		text, heading, chunk UNINDEXED, bucket UNINDEXED, document UNINDEXED, name UNINDEXED,
		tokenize = 'porter unicode61')`).Execute()
	return err
}

func dropTables(tx core.App) error {
	if _, err := tx.DB().NewQuery(`DROP TABLE IF EXISTS ` + FTSTable).Execute(); err != nil {
		return err
	}
	for _, name := range []string{ChunksCollection, DocumentsCollection, BucketsCollection} {
		c, err := tx.FindCollectionByNameOrId(name)
		if err != nil {
			continue
		}
		c.System = false // a system collection can't be deleted
		if err := tx.SaveNoValidate(c); err != nil {
			return err
		}
		if err := tx.Delete(c); err != nil {
			return err
		}
	}
	return nil
}
