package clicmdv1_test

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	clicmdv1 "github.com/pockethost/pocketone/ext/contracts/cli/commands/v1"
	examplev1 "github.com/pockethost/pocketone/ext/contracts/internal/example/v1"
	_ "github.com/pockethost/pocketone/ext/contracts/mail/feedback/v1" // public raw ops: no commands
	_ "github.com/pockethost/pocketone/ext/contracts/mail/suppression/v1"
	_ "github.com/pockethost/pocketone/ext/contracts/remote/v1"
	"github.com/pockethost/pocketone/ext/contracts/remote/v1/remoteclient"
	_ "github.com/pockethost/pocketone/ext/contracts/restore/v1"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/spf13/cobra"
)

func TestPathOf(t *testing.T) {
	for _, c := range []struct {
		id       extkit.ContractID
		op, want string
	}{
		{"remote/v1", "status", "remote status"},
		{"restore/v1", "restore_to", "remote restore to"},
		{"restore/v1", "points", "remote restore points"},
		{"mail.suppression/v1", "check", "remote mail suppression check"},
		{"rehash/v1", "rehash", "remote rehash"},
		{"settings/v1", "get", "remote settings get"},
	} {
		if got := clicmdv1.PathOf(c.id, c.op); got != c.want {
			t.Errorf("PathOf(%s, %s) = %q, want %q", c.id, c.op, got, c.want)
		}
	}
}

// ext is a toy extension.
type ext struct {
	name     string
	decl     extkit.Contracts
	register func(app core.App)
}

func (e *ext) Name() string                { return e.name }
func (e *ext) Contracts() extkit.Contracts { return e.decl }
func (e *ext) Register(app core.App, _ extkit.Config) error {
	e.register(app)
	return nil
}

type deployer struct{}

func (deployer) Manifest(context.Context, extkit.Auth) (examplev1.Manifest, error) {
	return examplev1.Manifest{Files: []examplev1.File{{Path: "pb_hooks/main.pb.js", Size: 10, Hash: "abc"}}}, nil
}
func (deployer) Upload(_ context.Context, _ extkit.Auth, in examplev1.UploadInput) (examplev1.UploadResult, error) {
	return examplev1.UploadResult{Written: len(in.Files), By: "toy"}, nil
}

func find(root *cobra.Command, path string) *cobra.Command {
	c, rest, err := root.Find(strings.Fields(path))
	if err != nil || len(rest) > 0 || c == root {
		return nil
	}
	return c
}

func newApp(t *testing.T, exts ...extkit.Extension) *tests.TestApp {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	if _, err := extkit.Setup(app, exts, nil, nil); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestMountBuildsTheTree(t *testing.T) {
	ran := ""
	rehasher := &ext{name: "rehasher",
		decl: extkit.Contracts{Implements: extkit.IDs("cli.commands/v1")},
		register: func(app core.App) {
			// replace one generated command, add one of its own
			clicmdv1.Contract.Provide(app, clicmdv1.Command{Path: "remote example upload", Short: "custom upload",
				Run: func(*clicmdv1.Ctx, []string) error { ran = "upload"; return nil }})
			clicmdv1.Contract.Provide(app, clicmdv1.Command{Path: "remote rehash", Short: "rehash everything",
				Run: func(*clicmdv1.Ctx, []string) error { ran = "rehash"; return nil }})
		}}
	app := newApp(t, rehasher)
	root := &cobra.Command{Use: "pocketone"}
	whoami := &cobra.Command{Use: "whoami", Short: "hand-written", Run: func(*cobra.Command, []string) {}}
	remote := &cobra.Command{Use: "remote"}
	remote.AddCommand(whoami)
	root.AddCommand(remote)
	clicmdv1.Mount(app, root)

	for _, p := range []string{"remote example manifest", "remote restore points", "remote restore to",
		"remote mail suppression check", "remote status", "remote rehash"} {
		if find(root, p) == nil {
			t.Errorf("no %q", p)
		}
	}
	for _, p := range []string{"remote mail feedback ses", "remote mail suppression unsubscribe-page"} {
		if find(root, p) != nil {
			t.Errorf("%q is for services and browsers, not a command", p)
		}
	}
	if c := find(root, "remote whoami"); c == nil || c.Short != "hand-written" {
		t.Errorf("a generated command replaced a hand-written one: %v", c)
	}
	if c := find(root, "remote example upload"); c == nil || c.Short != "custom upload" {
		t.Fatalf("the provided command should replace the generated one: %v", c)
	}
	if c := find(root, "remote restore to"); c == nil || c.Flags().Lookup("at") == nil || c.Flags().Lookup("remote") == nil {
		t.Fatalf("restore to: flags from its input (--at) and --remote: %v", c)
	}
	root.SetArgs([]string{"remote", "rehash"})
	if err := root.Execute(); err != nil || ran != "rehash" {
		t.Fatalf("rehash: %v (ran %q)", err, ran)
	}
}

func serve(t *testing.T, app core.App) *httptest.Server {
	t.Helper()
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.OnServe().Trigger(&core.ServeEvent{App: app, Router: r}, func(*core.ServeEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// A generated command calls the remote's contract, with its flags as the input.
func TestGeneratedCommandCallsTheRemote(t *testing.T) {
	toy := &ext{name: "toy",
		decl:     extkit.Contracts{Implements: extkit.IDs("example/v1")},
		register: func(app core.App) { examplev1.Contract.Provide(app, deployer{}) }}
	app := newApp(t, toy)
	srv := serve(t, app)
	su, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	tok, _ := su.NewAuthToken()
	t.Setenv(remoteclient.EnvVar("TOKEN"), tok)
	t.Chdir(t.TempDir()) // no project: --remote names the URL

	cli := newApp(t) // the developer's process: no providers, just the linked contracts
	run := func(args ...string) (string, error) {
		root := &cobra.Command{Use: "pocketone", SilenceErrors: true}
		clicmdv1.Mount(cli, root)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs(args)
		err := root.Execute()
		return out.String(), err
	}
	out, err := run("remote", "example", "manifest", "--remote", srv.URL)
	if err != nil || !strings.Contains(out, `"pb_hooks/main.pb.js"`) {
		t.Fatalf("manifest: %v\n%s", err, out)
	}
	out, err = run("remote", "example", "upload", "--remote", srv.URL, "--files", `[{"path":"a"},{"path":"b"}]`)
	if err != nil || !strings.Contains(out, `"written": 2`) {
		t.Fatalf("upload: %v\n%s", err, out)
	}
	// a remote without the contract says so
	_, err = run("remote", "restore", "points", "--remote", srv.URL)
	if err == nil || !strings.Contains(err.Error(), "doesn't offer restore/v1") {
		t.Fatalf("missing contract: %v", err)
	}
}
