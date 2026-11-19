package extkit

import (
	"errors"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

func newApp(t *testing.T) core.App {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	return app
}

func createCollection(name string) func(core.App) error {
	return func(tx core.App) error {
		c := core.NewBaseCollection(name)
		c.Fields.Add(&core.TextField{Name: "title"})
		return tx.Save(c)
	}
}

func dropCollection(name string) func(core.App) error {
	return func(tx core.App) error {
		c, err := tx.FindCollectionByNameOrId(name)
		if err != nil {
			return err
		}
		return tx.Delete(c)
	}
}

func hasCollection(app core.App, name string) bool {
	_, err := app.FindCollectionByNameOrId(name)
	return err == nil
}

var testMigrations = []Migration{
	// out of order on purpose: they run by name
	{Name: "0002_tags", Up: createCollection("_mx_tags"), Down: dropCollection("_mx_tags")},
	{Name: "0001_items", Up: createCollection("_mx_items"), Down: dropCollection("_mx_items")},
}

// Up applies pending migrations in name order and records them; a second run applies nothing; down reverts newest
// first, n at a time or all; status shows what's applied.
func TestMigrateUpDown(t *testing.T) {
	app := newApp(t)
	applied, err := MigrateUp(app, "mx", testMigrations)
	if err != nil || strings.Join(applied, ",") != "0001_items,0002_tags" {
		t.Fatalf("up: %v %v", applied, err)
	}
	if !hasCollection(app, "_mx_items") || !hasCollection(app, "_mx_tags") {
		t.Fatal("up didn't create the collections")
	}
	if again, err := MigrateUp(app, "mx", testMigrations); err != nil || len(again) != 0 {
		t.Fatalf("second up: %v %v", again, err)
	}
	st, err := MigrationStatus(app, "mx", testMigrations)
	if err != nil || len(st) != 2 || st[0].Name != "0001_items" || st[0].Applied == nil || st[1].Applied == nil {
		t.Fatalf("status: %+v %v", st, err)
	}
	// another extension's history is its own
	if st, _ := MigrationStatus(app, "other", testMigrations); st[0].Applied != nil {
		t.Fatal("another extension sees mx's history")
	}

	reverted, err := MigrateDown(app, "mx", testMigrations, 1)
	if err != nil || strings.Join(reverted, ",") != "0002_tags" || hasCollection(app, "_mx_tags") || !hasCollection(app, "_mx_items") {
		t.Fatalf("down 1: %v %v", reverted, err)
	}
	reverted, err = MigrateDown(app, "mx", testMigrations, 0)
	if err != nil || strings.Join(reverted, ",") != "0001_items" || hasCollection(app, "_mx_items") {
		t.Fatalf("down all: %v %v", reverted, err)
	}
	if applied, err := MigrateUp(app, "mx", testMigrations); err != nil || len(applied) != 2 {
		t.Fatalf("up after down: %v %v", applied, err)
	}
}

// A failing migration rolls back: nothing it did stays, it isn't recorded, and the ones after it don't run.
func TestMigrateFailureRollsBack(t *testing.T) {
	app := newApp(t)
	list := []Migration{
		{Name: "0001_ok", Up: createCollection("_mx_ok")},
		{Name: "0002_bad", Up: func(tx core.App) error {
			if err := createCollection("_mx_half")(tx); err != nil {
				return err
			}
			return errors.New("boom")
		}},
		{Name: "0003_after", Up: createCollection("_mx_after")},
	}
	applied, err := MigrateUp(app, "mx", list)
	if err == nil || !strings.Contains(err.Error(), "0002_bad") || strings.Join(applied, ",") != "0001_ok" {
		t.Fatalf("up: %v %v", applied, err)
	}
	if !hasCollection(app, "_mx_ok") || hasCollection(app, "_mx_half") || hasCollection(app, "_mx_after") {
		t.Fatal("the failed migration wasn't rolled back, or a later one ran")
	}
	if st, _ := MigrationStatus(app, "mx", list); st[1].Applied != nil {
		t.Fatal("the failed migration was recorded")
	}
}

func TestMigrationChecks(t *testing.T) {
	app := newApp(t)
	if _, err := MigrateUp(app, "mx", []Migration{{Name: "a", Up: createCollection("_mx_a")}, {Name: "a", Up: createCollection("_mx_b")}}); err == nil {
		t.Fatal("duplicate names: want an error")
	}
	if _, err := MigrateUp(app, "mx", []Migration{{Name: "", Up: createCollection("_mx_a")}}); err == nil {
		t.Fatal("empty name: want an error")
	}
	list := []Migration{{Name: "0001", Up: createCollection("_mx_one")}} // no Down
	if _, err := MigrateUp(app, "mx", list); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateDown(app, "mx", list, 0); err == nil || !strings.Contains(err.Error(), "can't be reverted") {
		t.Fatalf("down without Down: %v", err)
	}
}

type migratingExt struct{}

func (migratingExt) Name() string                    { return "mx" }
func (migratingExt) Register(core.App, Config) error { return nil }
func (migratingExt) Migrations() []Migration         { return testMigrations }

// An extension that's a Migrator gets its migrations applied when it's set up (here on an app that has already
// bootstrapped, so right away).
func TestSetupAppliesMigrations(t *testing.T) {
	app := newApp(t)
	if _, err := Setup(app, []Extension{migratingExt{}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !hasCollection(app, "_mx_items") || !hasCollection(app, "_mx_tags") {
		t.Fatal("setup didn't apply the extension's migrations")
	}
}

// On an app that hasn't bootstrapped yet, the migrations run during bootstrap, after PocketBase's own.
func TestMigrationsRunAtBootstrap(t *testing.T) {
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()})
	if _, err := Setup(app, []Extension{migratingExt{}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if !hasCollection(app, "_mx_items") || !hasCollection(app, "_mx_tags") {
		t.Fatal("bootstrap didn't apply the extension's migrations")
	}
	if !hasCollection(app, core.CollectionNameSuperusers) {
		t.Fatal("system migrations didn't run first")
	}
}

func TestRenameCollection(t *testing.T) {
	app := newApp(t)
	c := core.NewBaseCollection("_old", "pbc_mx_old")
	c.System = true
	c.Fields.Add(&core.TextField{Name: "hash"})
	c.AddIndex("idx_old_hash", true, "hash", "")
	if err := app.Save(c); err != nil {
		t.Fatal(err)
	}
	list := []Migration{RenameCollection("0001_rename", "pbc_mx_old", "_old", "_mx_new")}
	if _, err := MigrateUp(app, "mx", list); err != nil || !hasCollection(app, "_mx_new") || hasCollection(app, "_old") {
		t.Fatalf("up: %v", err)
	}
	renamed, _ := app.FindCollectionByNameOrId("pbc_mx_old")
	if !renamed.System || len(renamed.Indexes) != 1 || !strings.Contains(renamed.Indexes[0], "_mx_new") {
		t.Fatalf("renamed collection: system %v, indexes %v", renamed.System, renamed.Indexes)
	}
	r := core.NewRecord(renamed)
	r.Set("hash", "x")
	if err := app.Save(r); err != nil {
		t.Fatal("the renamed table doesn't work:", err)
	}
	if _, err := MigrateDown(app, "mx", list, 0); err != nil || !hasCollection(app, "_old") {
		t.Fatalf("down: %v", err)
	}
	// nothing to rename on a fresh app
	fresh := newApp(t)
	if _, err := MigrateUp(fresh, "mx", list); err != nil {
		t.Fatal(err)
	}
}
