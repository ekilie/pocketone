package standard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	_ "ganigeorgiev.com/pocketbase/pocketbase/migrations"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/spf13/cobra"
)

type migExt struct{}

func (migExt) Name() string                           { return "mig" }
func (migExt) Register(core.App, extkit.Config) error { return nil }
func (migExt) Migrations() []extkit.Migration {
	return []extkit.Migration{{Name: "0001_items",
		Up: func(tx core.App) error { return tx.Save(core.NewBaseCollection("_mig_items")) },
		Down: func(tx core.App) error {
			c, err := tx.FindCollectionByNameOrId("_mig_items")
			if err != nil {
				return err
			}
			return tx.Delete(c)
		}}}
}

func run(t *testing.T, cmds []*cobra.Command, args ...string) string {
	t.Helper()
	root := &cobra.Command{Use: "pocketone"}
	root.AddCommand(cmds...)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

func TestSetCommands(t *testing.T) {
	// a data dir of its own: pb_cache goes next to it
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: filepath.Join(t.TempDir(), "pb_data")})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	plan := &extkit.Plan{On: []extkit.Extension{migExt{}}}
	cmds := setCommands(app, plan)

	if out := run(t, cmds, "extensions", "migrate", "up"); !strings.Contains(out, "applied mig 0001_items") {
		t.Fatalf("up: %s", out)
	}
	if out := run(t, cmds, "extensions", "migrations"); !strings.Contains(out, "mig") || strings.Contains(out, "pending") {
		t.Fatalf("status: %s", out)
	}
	if out := run(t, cmds, "extensions", "migrate", "down", "mig"); !strings.Contains(out, "reverted mig 0001_items") {
		t.Fatalf("down: %s", out)
	}
	if out := run(t, cmds, "extensions", "migrations"); !strings.Contains(out, "pending") {
		t.Fatalf("status after down: %s", out)
	}

	dir, err := extkit.CacheDir(app, "agent")
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "x.bin"), []byte("12345"), 0o644)
	if out := run(t, cmds, "cache"); !strings.Contains(out, "agent") || !strings.Contains(out, "5 B") {
		t.Fatalf("cache: %s", out)
	}
	if out := run(t, cmds, "cache", "clear"); !strings.Contains(out, "cleared pb_cache/agent") {
		t.Fatalf("cache clear: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "x.bin")); !os.IsNotExist(err) {
		t.Fatal("file survived cache clear")
	}
}
