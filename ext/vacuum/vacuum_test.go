package vacuum

import (
	"os"
	"path/filepath"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// bloated writes a data.db with 2 MB of rows, then deletes them: VACUUM has something to give back.
func bloated(t *testing.T, appDir string) string {
	t.Helper()
	path := filepath.Join(appDir, "pb_data", "data.db")
	os.MkdirAll(filepath.Dir(path), 0o755)
	db, err := core.DefaultDBConnect(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.NewQuery("CREATE TABLE t (b BLOB)").Execute(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		if _, err := db.NewQuery("INSERT INTO t VALUES (randomblob(10000))").Execute(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.NewQuery("DELETE FROM t").Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NewQuery("PRAGMA wal_checkpoint(TRUNCATE)").Execute(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDirShrinksTheDatabase(t *testing.T) {
	dir := t.TempDir()
	path := bloated(t, dir)
	before, _ := size(path)
	saved, err := Dir(dir)
	after, _ := size(path)
	if err != nil || saved < 1_000_000 || after >= before {
		t.Fatalf("Dir: saved %d, %d -> %d bytes, %v", saved, before, after, err)
	}
	if saved, err := Dir(t.TempDir()); err != nil || saved != 0 {
		t.Fatalf("an app folder with no databases: %d %v", saved, err)
	}
}

func TestAppOnline(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if err := App(app); err != nil {
		t.Fatal(err)
	}
}
