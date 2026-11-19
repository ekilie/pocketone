package sandbox

import (
	"path/filepath"
	"testing"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"pocketone.dev/ext/extkit"
)

// The guarded database speaks SQLite's dialect: dbx picks it by driver name, and its generic builder writes
// statements SQLite rejects, such as RENAME TABLE for a collection rename (every Pocker instance's database opens
// through the guard).
func TestGuardedDBUsesSQLiteBuilder(t *testing.T) {
	db, err := GuardedDBConnect(0)(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, ok := db.Builder.(*dbx.SqliteBuilder); !ok {
		t.Fatalf("builder is %T, want *dbx.SqliteBuilder", db.Builder)
	}
	if _, err := db.NewQuery("CREATE TABLE a (x TEXT)").Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RenameTable("a", "b").Execute(); err != nil {
		t.Fatalf("rename: %v", err)
	}
	var n int
	if err := db.NewQuery("SELECT count(*) FROM sqlite_schema WHERE name = 'b'").Row(&n); err != nil || n != 1 {
		t.Fatalf("renamed table not found (%d, %v)", n, err)
	}
}

// A collection rename (an extension's rename migration, as API Tokens' 0001_rename_tokens) works on a guarded app.
func TestGuardedAppRenamesCollection(t *testing.T) {
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir(), DBConnect: GuardedDBConnect(0)})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunSystemMigrations(); err != nil {
		t.Fatal(err)
	}
	c := core.NewBaseCollection("_tokens", "pbc_tokens")
	c.System = true
	c.Fields.Add(&core.TextField{Name: "hash"})
	c.AddIndex("idx_tokens_hash", true, "hash", "")
	if err := app.Save(c); err != nil {
		t.Fatal(err)
	}
	list := []extkit.Migration{extkit.RenameCollection("0001_rename_tokens", "pbc_tokens", "_tokens", "_api_tokens_tokens")}
	if _, err := extkit.MigrateUp(app, "api-tokens", list); err != nil {
		t.Fatalf("rename migration: %v", err)
	}
	if _, err := app.FindCollectionByNameOrId("_api_tokens_tokens"); err != nil {
		t.Fatal("not renamed:", err)
	}
}
