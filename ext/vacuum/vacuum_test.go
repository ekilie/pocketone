package vacuum

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"

	settingsv1 "github.com/pockethost/pocketone/ext/contracts/settings/v1"
	vacuumv1 "github.com/pockethost/pocketone/ext/contracts/vacuum/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

func testApp(t *testing.T) core.App {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	return app
}

// bloat writes 4 MB of rows into the app's data.db and deletes them: free pages VACUUM can give back.
func bloat(t *testing.T, app core.App) {
	t.Helper()
	for _, q := range []string{"CREATE TABLE bloat (b BLOB)", "WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 400) INSERT INTO bloat SELECT randomblob(10000) FROM n", "DELETE FROM bloat", "PRAGMA wal_checkpoint(TRUNCATE)"} {
		if _, err := app.DB().NewQuery(q).Execute(); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// A database with little free space is left alone; one with lots is compacted, online, and the space comes back.
func TestAppCompactsOnlyWhenWorthIt(t *testing.T) {
	app := testApp(t)
	run := App(app)
	if run.Error != "" || run.Saved != 0 || !strings.Contains(strings.Join(run.Skipped, ","), "data.db") {
		t.Fatalf("a fresh app: %+v", run)
	}
	bloat(t, app)
	run = App(app)
	if run.Error != "" || run.Saved < 1_000_000 || strings.Contains(strings.Join(run.Skipped, ","), "data.db") {
		t.Fatalf("after deleting 4 MB: %+v", run)
	}
}

var su = extkit.Auth{Kind: "superuser"}

// The admins' settings: off by default without a pocketone.yaml section, saved through settings/v1, reflected in
// vacuum/v1's status; a bad schedule is refused.
func TestSettingsAndStatus(t *testing.T) {
	app := testApp(t)
	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	v, ok := vacuumv1.Contract.Of(app)
	if !ok {
		t.Fatal("no vacuum/v1")
	}
	st, err := v.Status(context.Background(), su)
	if err != nil || st.Provider != Name || st.Enabled || st.Every != "168h" {
		t.Fatalf("default status: %+v %v", st, err)
	}
	var section settingsv1.Section
	for _, s := range settingsv1.Sections(app) {
		if s.Info().Extension == Name {
			section = s
		}
	}
	saver, ok := section.(settingsv1.Saver)
	if !ok {
		t.Fatal("no vacuum settings section to save")
	}
	if _, err := saver.Save(context.Background(), json.RawMessage(`{"enabled": true, "every": "24h"}`)); err != nil {
		t.Fatal(err)
	}
	st, _ = v.Status(context.Background(), su)
	if !st.Enabled || st.Every != "24h" {
		t.Fatalf("status after saving: %+v", st)
	}
	if _, err := saver.Save(context.Background(), json.RawMessage(`{"enabled": true, "every": "5m"}`)); err == nil {
		t.Fatal("a schedule under an hour was accepted")
	}
}

// hostVacuum is a host's vacuum/v1 for its instances.
type hostVacuum struct{ id string }

func (h hostVacuum) Status(context.Context, extkit.Auth) (vacuumv1.Status, error) {
	return vacuumv1.Status{Provider: "host", Enabled: true, Every: "24h"}, nil
}

// When the app's host provides vacuum/v1 for its instances, the app half steps aside: no settings section for the
// app's admins, no schedule, and the contract answers with the host's status.
func TestHostTakesOver(t *testing.T) {
	app := testApp(t)
	host := extkit.NewScope("multisite")
	vacuumv1.Contract.ProvideForInstances(host, func(id string) vacuumv1.Vacuum { return hostVacuum{id} })
	extkit.AttachHost(app, host, "a")

	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	for _, s := range settingsv1.Sections(app) {
		if s.Info().Extension == Name {
			t.Fatal("the app's admins got a vacuum setting on a host that owns vacuum")
		}
	}
	if err := extkit.AppScope(app).Seal(nil); err != nil { // as Pocker does once an instance's extensions registered
		t.Fatal(err)
	}
	v, ok := vacuumv1.Contract.Of(app)
	if !ok {
		t.Fatal("no vacuum/v1 on the instance")
	}
	if st, err := v.Status(context.Background(), su); err != nil || st.Provider != "host" {
		t.Fatalf("status on the instance: %+v %v", st, err)
	}
}
