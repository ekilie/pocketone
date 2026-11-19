package sandbox

import (
	"path/filepath"
	"testing"

	"ganigeorgiev.com/pocketbase/dbx"
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
