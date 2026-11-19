package extkit

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
)

// Extension migrations: each extension owns its schema through native Go migrations, applied and recorded apart
// from the app's pb_migrations.
//
//   - When: at bootstrap, right after PocketBase's own system migrations, on every app the extension runs on (each
//     isolate has its own database). So an extension's tables exist for every command, before the app's pb_migrations
//     run at serve, and an app migration may refer to them.
//   - Order: within an extension, by Name (zero-pad numbers: "0001_chats"); across extensions, registration order,
//     which extkit sorts by contracts (a provider before what requires it).
//   - History: the _extension_migrations table (extension, name, applied), apart from the app's _migrations, so the
//     app's `migrate down` and `history-sync` never touch extension tables, and an extension that's off keeps its
//     history.
//   - Each migration runs in a transaction, like PocketBase's.

// Migration is one step of an extension's schema. Up and Down get the transaction's app.
type Migration struct {
	Name string // unique in the extension; applied in name order ("0001_chats", "0002_meta_index")
	Up   func(txApp core.App) error
	Down func(txApp core.App) error // nil: the migration can't be reverted
}

// Migrator is an extension with migrations.
type Migrator interface {
	Migrations() []Migration
}

// MigrationsTable records which extension migrations an app has applied.
const MigrationsTable = "_extension_migrations"

// MigrationState is one migration and when it was applied (nil: pending).
type MigrationState struct {
	Name    string
	Applied *time.Time
}

// checkMigrations validates an extension's list: names set and unique.
func checkMigrations(ext string, list []Migration) error {
	seen := map[string]bool{}
	for _, m := range list {
		if m.Name == "" || m.Up == nil {
			return fmt.Errorf("extkit: %s: every migration needs a Name and an Up", ext)
		}
		if seen[m.Name] {
			return fmt.Errorf("extkit: %s: two migrations are named %q", ext, m.Name)
		}
		seen[m.Name] = true
	}
	return nil
}

func sorted(list []Migration) []Migration {
	out := append([]Migration(nil), list...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func ensureMigrationsTable(app core.App) error {
	_, err := app.DB().NewQuery("CREATE TABLE IF NOT EXISTS {{" + MigrationsTable + "}} (" +
		"[[extension]] TEXT NOT NULL, [[name]] TEXT NOT NULL, [[applied]] INTEGER NOT NULL, " +
		"PRIMARY KEY ([[extension]], [[name]]))").Execute()
	return err
}

func appliedMigrations(app core.App, ext string) (map[string]time.Time, error) {
	if err := ensureMigrationsTable(app); err != nil {
		return nil, err
	}
	var rows []struct {
		Name    string `db:"name"`
		Applied int64  `db:"applied"`
	}
	err := app.DB().Select("name", "applied").From(MigrationsTable).Where(dbx.HashExp{"extension": ext}).All(&rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	out := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		out[r.Name] = time.UnixMicro(r.Applied)
	}
	return out, nil
}

// MigrateUp applies ext's pending migrations on app, in name order, each in its own transaction; it returns the names
// it applied.
func MigrateUp(app core.App, ext string, list []Migration) ([]string, error) {
	if err := checkMigrations(ext, list); err != nil {
		return nil, err
	}
	done, err := appliedMigrations(app, ext)
	if err != nil {
		return nil, err
	}
	var applied []string
	for _, m := range sorted(list) {
		if _, ok := done[m.Name]; ok {
			continue
		}
		err := app.RunInTransaction(func(tx core.App) error {
			if err := m.Up(tx); err != nil {
				return err
			}
			_, err := tx.DB().Insert(MigrationsTable, dbx.Params{
				"extension": ext, "name": m.Name, "applied": time.Now().UnixMicro(),
			}).Execute()
			return err
		})
		if err != nil {
			return applied, fmt.Errorf("extension %s: migration %s: %w", ext, m.Name, err)
		}
		applied = append(applied, m.Name)
	}
	return applied, nil
}

// MigrateDown reverts ext's last n applied migrations on app (newest first; n <= 0: all of them), each in its own
// transaction; it returns the names it reverted. A migration without Down stops it with an error.
func MigrateDown(app core.App, ext string, list []Migration, n int) ([]string, error) {
	if err := checkMigrations(ext, list); err != nil {
		return nil, err
	}
	done, err := appliedMigrations(app, ext)
	if err != nil {
		return nil, err
	}
	all := sorted(list)
	var reverted []string
	for i := len(all) - 1; i >= 0; i-- {
		m := all[i]
		if _, ok := done[m.Name]; !ok {
			continue
		}
		if n > 0 && len(reverted) == n {
			break
		}
		if m.Down == nil {
			return reverted, fmt.Errorf("extension %s: migration %s can't be reverted", ext, m.Name)
		}
		err := app.RunInTransaction(func(tx core.App) error {
			if err := m.Down(tx); err != nil {
				return err
			}
			_, err := tx.DB().Delete(MigrationsTable, dbx.HashExp{"extension": ext, "name": m.Name}).Execute()
			return err
		})
		if err != nil {
			return reverted, fmt.Errorf("extension %s: reverting %s: %w", ext, m.Name, err)
		}
		reverted = append(reverted, m.Name)
	}
	return reverted, nil
}

// MigrationStatus lists ext's migrations in order, with when each was applied on app.
func MigrationStatus(app core.App, ext string, list []Migration) ([]MigrationState, error) {
	done, err := appliedMigrations(app, ext)
	if err != nil {
		return nil, err
	}
	out := make([]MigrationState, 0, len(list))
	for _, m := range sorted(list) {
		st := MigrationState{Name: m.Name}
		if t, ok := done[m.Name]; ok {
			st.Applied = &t
		}
		out = append(out, st)
	}
	return out, nil
}

// bindMigrations applies every Migrator's migrations on app at bootstrap, after PocketBase's own system migrations,
// in exts' order (or right away when app has already bootstrapped).
func bindMigrations(app core.App, exts []Extension) error {
	type pending struct {
		name string
		list []Migration
	}
	var all []pending
	for _, e := range exts {
		m, ok := e.(Migrator)
		if !ok {
			continue
		}
		list := m.Migrations()
		if err := checkMigrations(e.Name(), list); err != nil {
			return err
		}
		all = append(all, pending{e.Name(), list})
	}
	if len(all) == 0 {
		return nil
	}
	run := func(app core.App) error {
		for _, p := range all {
			applied, err := MigrateUp(app, p.name, p.list)
			if err != nil {
				return err
			}
			if len(applied) > 0 {
				app.Logger().Info("extension migrations applied", "extension", p.name, "migrations", applied)
			}
		}
		return nil
	}
	if app.IsBootstrapped() {
		return run(app)
	}
	app.OnBootstrap().Bind(&hook.Handler[*core.BootstrapEvent]{
		Id: "__extkitMigrations__",
		Func: func(e *core.BootstrapEvent) error {
			if err := e.Next(); err != nil { // the database is open and PocketBase's system migrations have run
				return err
			}
			return run(e.App)
		},
	})
	return nil
}

// RenameCollection is a migration that renames the collection with id from one name to another (Down renames it
// back). A collection that doesn't exist yet, or already has the new name, is left alone: the extension creates it
// under the new name. Keep the id: code that finds the collection by id keeps working.
func RenameCollection(name, id, from, to string) Migration {
	rename := func(oldName, newName string) func(core.App) error {
		return func(tx core.App) error {
			c, err := tx.FindCollectionByNameOrId(id)
			if err != nil || c.Name != oldName {
				return nil
			}
			c.Name = newName
			// PocketBase's validation refuses to rename a system collection (an extension's usually are); the save
			// itself renames the table and its indexes
			return tx.SaveNoValidate(c)
		}
	}
	return Migration{Name: name, Up: rename(from, to), Down: rename(to, from)}
}
