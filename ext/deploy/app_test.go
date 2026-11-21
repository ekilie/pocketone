package deploy

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	deployv1 "pocketone.dev/ext/contracts/deploy/v1"
	"pocketone.dev/ext/remotes"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"
	"pocketone.dev/ext/api-tokens"
	remoteclient "pocketone.dev/ext/contracts/remote/v1/remoteclient"
	"pocketone.dev/ext/extkit"
)

// serveApp runs app's router (OnServe included) on a local test server.
func serveApp(t *testing.T, app *tests.TestApp) string {
	t.Helper()
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	se := &core.ServeEvent{App: app, Router: router}
	var srv *httptest.Server
	err = app.OnServe().Trigger(se, func(e *core.ServeEvent) error {
		mux, err := e.Router.BuildMux()
		if err != nil {
			return err
		}
		srv = httptest.NewServer(mux)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv.URL
}

type p1Remote struct {
	app      *tests.TestApp
	url      string
	dir      string
	restarts atomic.Int32
	tk       *apitokens.Tokens
}

func newP1Remote(t *testing.T) *p1Remote { return newP1RemoteWith(t, nil) }

// newP1RemoteWith lets a test adjust the deploy Config (Dir and Restart are set first).
func newP1RemoteWith(t *testing.T, adjust func(*Config)) *p1Remote {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	r := &p1Remote{app: app, dir: t.TempDir()}
	count := func(core.App) error { r.restarts.Add(1); return nil }
	cfg := Config{Dir: r.dir, Restart: count}
	if adjust != nil {
		adjust(&cfg)
	}
	// registered and sealed the way every app is (the standard set), so remote/v1 (restart, logs) mounts
	exts := []extkit.Extension{
		apitokens.New(apitokens.Config{Collections: []string{"_superusers", "users"}}),
		remotes.New(remotes.Config{Restart: count}),
		NewExtension(cfg),
	}
	if _, err := extkit.Setup(app, exts, nil, nil); err != nil {
		t.Fatal(err)
	}
	r.tk = apitokens.For(app)
	// the test app is already bootstrapped: run what OnBootstrap would
	if err := app.OnBootstrap().Trigger(&core.BootstrapEvent{App: app}, func(e *core.BootstrapEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := apitokens.EnsureSchema(app); err != nil {
		t.Fatal(err)
	}
	r.url = serveApp(t, app)
	return r
}

func (r *p1Remote) token(t *testing.T, collection, email string, scopes ...string) string {
	t.Helper()
	owner, err := r.app.FindAuthRecordByEmail(collection, email)
	if err != nil {
		t.Fatal(err)
	}
	secret, _, err := r.tk.Create(owner, apitokens.CreateOptions{Name: "ci", Scopes: scopes})
	if err != nil {
		t.Fatal(err)
	}
	return secret
}

func (r *p1Remote) client(t *testing.T, token string) *Client {
	t.Helper()
	rc := &remoteclient.Client{Remote: &remoteclient.Remote{Name: "ce", URL: r.url}, Token: token, TokenSource: "test", HTTP: remoteclient.NewHTTPClient()}
	c, err := NewClient(context.Background(), rc)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestP1ToP1(t *testing.T) {
	r := newP1Remote(t)
	ctx := context.Background()
	c := r.client(t, r.token(t, "_superusers", "test@example.com", ScopeDeploy, "remote", "remote:read"))
	if c.Prefix != deployv1.Prefix {
		t.Fatalf("discovered %q", c.Prefix)
	}
	if w, err := c.Whoami(ctx); err != nil || w.Via != "token" {
		t.Fatalf("whoami %+v %v", w, err)
	}

	project := t.TempDir()
	put := func(rel, content string) {
		p := filepath.Join(project, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	put("pb_hooks/main.pb.js", "routerAdd()")
	put("pb_public/index.html", "v1")
	put("pb_data/data.db", "local")
	deploy := func() *CommitResult {
		t.Helper()
		res, err := Deploy(ctx, c, DeployOptions{CollectOptions: CollectOptions{Dir: project}})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := deploy()
	// the app booted without pb_hooks, so nothing watches it: the deploy says a restart is needed, and doesn't do it
	if res.WrittenCount != 2 || res.Restarted || !res.RestartNeeded || r.restarts.Load() != 0 {
		t.Fatalf("first %+v restarts %d", res, r.restarts.Load())
	}
	if rr, err := c.Restart(ctx); err != nil || !rr.Restarted || !r.waitRestarts(1) {
		t.Fatalf("restart %+v %v (restarts %d)", rr, err, r.restarts.Load())
	}
	if b, _ := os.ReadFile(filepath.Join(r.dir, "pb_hooks/main.pb.js")); string(b) != "routerAdd()" {
		t.Fatal("hook not written")
	}
	if _, err := os.Stat(filepath.Join(r.dir, "pb_data/data.db")); err == nil {
		t.Fatal("pb_data shipped")
	}
	put("pb_public/index.html", "v2")
	if res = deploy(); res.WrittenCount != 1 || res.Restarted || r.restarts.Load() != 1 {
		t.Fatalf("pb_public-only deploy restarted: %+v", res)
	}
	if !deploy().NothingChanged {
		t.Fatal("no-op")
	}
	if _, err := r.app.FindCollectionByNameOrId("_deploy_deploys"); err == nil {
		t.Fatal("deploy history collection still exists")
	}

	// read-only token: pull and logs, no deploys
	ro := r.client(t, r.token(t, "_superusers", "test@example.com", ScopeRead, "remote:read"))
	if _, err := ro.Check(ctx, map[string]string{}, false); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("read-only check: %v", err)
	}
	if _, err := ro.Restart(ctx); err == nil || r.restarts.Load() != 1 {
		t.Fatalf("read-only restart: %v", err)
	}
	pullDir := t.TempDir()
	if n, err := Pull(ctx, ro, PullOptions{Dir: pullDir}); err != nil || n != 2 {
		t.Fatalf("pull %d %v", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(pullDir, "pb_public/index.html")); string(b) != "v2" {
		t.Fatal("pulled content")
	}

	// logs come from the app's _logs
	l := &core.Log{Message: "hello from the app", Level: 0}
	l.Data = map[string]any{"k": "v"}
	l.Id = core.GenerateDefaultRandomId()
	l.Created = types.NowDateTime()
	if err := r.app.AuxSave(l); err != nil {
		t.Fatal(err)
	}
	lctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var got []string
	ro.StreamLogs(lctx, 5, func(e LogEntry) {
		if strings.Contains(e.Message, "hello from the app") {
			got = append(got, e.Message)
			cancel()
		}
	})
	if len(got) == 0 {
		t.Fatal("no app log line")
	}

	// who may not deploy
	user := r.client(t, r.token(t, "users", "test@example.com"))
	if _, err := user.Check(ctx, map[string]string{}, false); err == nil || !strings.Contains(err.Error(), "Only superusers") {
		t.Fatalf("user token: %v", err)
	}
	anon := r.client(t, "")
	if _, err := anon.Check(ctx, map[string]string{}, false); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("anonymous: %v", err)
	}
	other := r.token(t, "_superusers", "test@example.com")
	if _, err := r.client(t, other).Check(ctx, map[string]string{}, false); err != nil {
		t.Fatalf("unscoped superuser token: %v", err)
	}
}

func TestP1ToP1WithImpersonateLogin(t *testing.T) {
	r := newP1Remote(t)
	rc := &remoteclient.Client{Remote: &remoteclient.Remote{Name: "ce", URL: r.url}, HTTP: remoteclient.NewHTTPClient()}
	su, _ := r.app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test@example.com")
	jwt, err := su.NewStaticAuthToken(time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	rc.Token = jwt
	c, err := NewClient(context.Background(), rc)
	if err != nil {
		t.Fatal(err)
	}
	if w, err := c.Whoami(context.Background()); err != nil || w.Via != "jwt" {
		t.Fatalf("%+v %v", w, err)
	}
}

// The standard binary serves pb_public at "GET /{path...}"; the deploy routes must not conflict with it.
func TestRoutesCoexistWithStaticCatchAll(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	MustRegister(app, Config{Dir: t.TempDir(), Restart: func(core.App) error { return nil }})
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/{path...}", func(e *core.RequestEvent) error { return e.String(200, "static") })
		return se.Next()
	})
	router, _ := apis.NewRouter(app)
	err = app.OnServe().Trigger(&core.ServeEvent{App: app, Router: router}, func(e *core.ServeEvent) error {
		_, err := e.Router.BuildMux()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// waitRestarts waits (briefly) for remote/v1's restart, which runs just after it answers.
func (r *p1Remote) waitRestarts(n int32) bool {
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if r.restarts.Load() == n {
			return true
		}
	}
	return false
}

// TestDropDeploysMigration: an app that had deploy history (under its old name, as a system collection with rows)
// loses the collection when the extension's migrations run.
func TestDropDeploysMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	c := core.NewBaseCollection("_deploys", deploysCollectionId)
	c.System = true
	c.Fields.Add(&core.TextField{Name: "status"})
	if err := app.Save(c); err != nil {
		t.Fatal(err)
	}
	rec := core.NewRecord(c)
	rec.Set("status", "success")
	if err := app.Save(rec); err != nil {
		t.Fatal(err)
	}
	applied, err := extkit.MigrateUp(app, Name, NewExtension(Config{}).Migrations())
	if err != nil || len(applied) != 2 {
		t.Fatalf("migrate: %v %v", applied, err)
	}
	if _, err := app.FindCollectionByNameOrId(deploysCollectionId); err == nil {
		t.Fatal("deploy history collection still exists")
	}
	// a fresh app (no collection) migrates cleanly too
	fresh, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Cleanup()
	if _, err := extkit.MigrateUp(fresh, Name, NewExtension(Config{}).Migrations()); err != nil {
		t.Fatal(err)
	}
}
