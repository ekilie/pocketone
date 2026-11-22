// Package conformance is the shared test every PocketOne extension runs. It checks what any [extkit.Extension] must get
// right whatever it does: it registers and runs on a fresh app and shuts down cleanly, it leaves PocketBase's own
// collections alone and names its own `_<name>_<table>` (hyphens in the name as underscores), applies, re-applies and reverts its migrations cleanly, one extension value can serve many apps at once (as under an isolate runtime)
// without one app's registration reaching another, and any MCP tools it adds are named `<name>_<verb>_<noun>` and
// described.
//
// An extension's test file calls it once:
//
//	func TestConformance(t *testing.T) {
//		conformance.Run(t, func() extkit.Extension { return myextension.New(myextension.Config{}) })
//	}
//
// Run it with -race: the many-apps check registers the same extension value on several apps concurrently. Feature tests
// stay in the extension's own test files; this is only the minimum.
//
// An extension that adds MCP tools (ext/mcp) puts an MCP server on the test apps with [Options.Setup], so the tool check
// can see them:
//
//	conformance.Run(t, newExtension, conformance.Options{Setup: func(app core.App) error {
//		_, err := mcp.Register(app, mcp.Config{NoBuiltins: true})
//		return err
//	}})
package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/router"
	"net/http"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	_ "ganigeorgiev.com/pocketbase/pocketbase/migrations" // PocketBase's own system migrations
	"pocketone.dev/ext/extkit"
)

// Options adjust Run for one extension.
type Options struct {
	// Layers are the settings layers passed to Register (default: none, the extension's own defaults).
	Layers extkit.Config

	// AllowCollections lists collections the extension may create without the `_<name>_` prefix (hyphens in the name as
	// underscores), each with a reason
	// a reviewer can check (e.g. a collection the app's own users are meant to see).
	AllowCollections map[string]string

	// Apps is how many apps the many-apps check registers the extension on (default 3).
	Apps int

	// GoroutineSlack is how many goroutines may outlive the app (default 0). Raise it only with a comment saying
	// which goroutine stays and why it's harmless.
	GoroutineSlack int

	// Setup runs on every test app before the extension registers: what the extension expects the host to have put there
	// first, such as an MCP server (ext/mcp) for an extension that adds tools.
	Setup func(app core.App) error

	// AllowRoutes lists routes (path patterns, e.g. "/{path...}") the extension may register outside the allowed
	// prefixes, each with a reason a reviewer can check.
	AllowRoutes map[string]string

	// Deps are extensions the one under test requires (its Contracts().Requires), registered first on every test app,
	// the way the standard set orders them: e.g. tokens for the OAuth server.
	Deps []extkit.Extension
}

// TB is the part of testing.T the checks use, so they can be tested against a recorder.
type TB interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Fatal(args ...any)
	Failed() bool
	TempDir() string
}

// Run runs every check as a subtest. newExtension must return a fresh extension value each call.
func Run(t *testing.T, newExtension func() extkit.Extension, opts ...Options) {
	t.Helper()
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}

	name := newExtension().Name()
	if name == "" || strings.ContainsAny(name, " _/.") || strings.ToLower(name) != name {
		t.Fatalf("Name() = %q: want a non-empty lowercase key with no spaces, dots, slashes or underscores", name)
	}
	if name == "pb" {
		t.Fatalf(`Name() = "pb": reserved for PocketBase's own MCP tools (pb_*)`)
	}

	t.Run("Lifecycle", func(t *testing.T) { CheckLifecycle(t, newExtension(), o) })
	t.Run("Collections", func(t *testing.T) { CheckCollections(t, newExtension(), o) })
	t.Run("Migrations", func(t *testing.T) {
		if _, ok := newExtension().(extkit.Migrator); !ok {
			t.Skip("not an extkit.Migrator: the extension has no migrations")
		}
		CheckMigrations(t, newExtension(), o)
	})
	t.Run("ManyApps", func(t *testing.T) { CheckManyApps(t, newExtension(), withApps(o)) })
	t.Run("Routes", func(t *testing.T) { CheckRoutes(t, newExtension(), o) })
	t.Run("MCPTools", func(t *testing.T) {
		if o.Setup == nil {
			t.Skip("no Options.Setup: the extension adds no MCP tools")
		}
		CheckMCPTools(t, newExtension(), o)
	})
}

func withApps(o Options) Options {
	if o.Apps <= 0 {
		o.Apps = 3
	}
	return o
}

// CheckLifecycle: Register, bootstrap, migrations, serve hooks and terminate all succeed, and nothing the extension
// started outlives the app (and, for an [extkit.Closer], its Close).
func CheckLifecycle(t TB, p extkit.Extension, o Options) {
	before := goroutines()
	app := newApp(t, o)
	if err := start(app, p, o.Layers); err != nil {
		t.Fatal(err)
	}
	if err := stop(app); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if c, ok := p.(extkit.Closer); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.Close(ctx)
		cancel()
		if err != nil {
			t.Errorf("Close: %v", err)
		}
	}
	if leaked := waitGoroutines(before, o.GoroutineSlack, 2*time.Second); len(leaked) > 0 {
		t.Errorf("%d goroutine(s) outlived the app (stop them in OnTerminate, or in Close for shared ones):\n\n%s", len(leaked),
			strings.Join(leaked, "\n\n"))
	}
}

// TablePrefix is the prefix every collection an extension adds starts with: `_<name>_`, with hyphens in the name as
// underscores (llm-chat: `_llm_chat_`).
func TablePrefix(name string) string { return "_" + strings.ReplaceAll(name, "-", "_") + "_" }

// CheckCollections: PocketBase's own collections are unchanged, and every collection the extension adds is named
// `_<name>_<table>` (see [TablePrefix]; or allowed with a reason).
func CheckCollections(t TB, p extkit.Extension, o Options) {
	base := newApp(t, o)
	if err := start(base, nil, nil); err != nil {
		t.Fatal(err)
	}
	want := snapshot(t, base)
	_ = stop(base)

	app := newApp(t, o)
	if err := start(app, p, o.Layers); err != nil {
		t.Fatal(err)
	}
	defer stop(app)
	got := snapshot(t, app)

	for name, def := range want {
		if got[name] == "" {
			t.Errorf("collection %q was removed", name)
		} else if got[name] != def {
			t.Errorf("collection %q was changed; extensions must not alter PocketBase's own collections", name)
		}
	}
	prefix := TablePrefix(p.Name())
	var added []string
	for name := range got {
		if _, ok := want[name]; !ok {
			added = append(added, name)
		}
	}
	sort.Strings(added)
	for _, name := range added {
		if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
			continue
		}
		if reason, ok := o.AllowCollections[name]; ok && reason != "" {
			continue
		}
		t.Errorf("collection %q: extension collections must be named %s<table> (or listed in AllowCollections with a reason)",
			name, prefix)
	}
}

// CheckMigrations, for an [extkit.Migrator]: its migrations are applied at bootstrap, applying again does nothing,
// reverting all of them changes the schema, and applying them again brings back exactly the schema they made. A list
// with an irreversible migration (no Down) skips the revert steps.
func CheckMigrations(t TB, p extkit.Extension, o Options) {
	m, ok := p.(extkit.Migrator)
	if !ok {
		return
	}
	list := m.Migrations()
	app := newApp(t, o)
	if err := start(app, p, o.Layers); err != nil {
		t.Fatal(err)
	}
	defer stop(app)
	st, err := extkit.MigrationStatus(app, p.Name(), list)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range st {
		if s.Applied == nil {
			t.Errorf("migration %s wasn't applied at bootstrap", s.Name)
		}
	}
	if again, err := extkit.MigrateUp(app, p.Name(), list); err != nil || len(again) > 0 {
		t.Errorf("applying again: applied %v, err %v; want nothing", again, err)
	}
	for _, mg := range list {
		if mg.Down == nil {
			return // can't revert: nothing more to check
		}
	}
	applied := snapshot(t, app)
	if _, err := extkit.MigrateDown(app, p.Name(), list, 0); err != nil {
		t.Fatalf("reverting all: %v", err)
	}
	if reflect.DeepEqual(snapshot(t, app), applied) {
		t.Errorf("reverting every migration left the schema as it was: a Down doesn't undo its Up")
	}
	if again, err := extkit.MigrateUp(app, p.Name(), list); err != nil || len(again) != len(list) {
		t.Errorf("applying after reverting: applied %v, err %v; want all %d", again, err, len(list))
	}
	after := snapshot(t, app)
	for name, def := range applied {
		if after[name] == "" {
			t.Errorf("collection %q is missing after reverting and applying again", name)
		} else if after[name] != def {
			t.Errorf("collection %q differs after reverting and applying again:\n%s\n%s", name, def, after[name])
		}
	}
	for name := range after {
		if applied[name] == "" {
			t.Errorf("collection %q appeared after reverting and applying again", name)
		}
	}
}

// CheckManyApps: one extension value registers on several apps in one process, as an isolate runtime does. Registering on one app must add
// nothing to any other app's hooks; registering concurrently must work (run with -race); and every app then runs.
func CheckManyApps(t TB, p extkit.Extension, o Options) {
	o = withApps(o)
	apps := make([]*core.BaseApp, o.Apps)
	for i := range apps {
		apps[i] = newApp(t, o)
	}
	for i, app := range apps {
		before := allHookCounts(apps)
		if err := p.Register(app, o.Layers); err != nil {
			t.Fatalf("Register on app %d: %v", i, err)
		}
		after := allHookCounts(apps)
		for j := range apps {
			if j == i {
				continue
			}
			for k, v := range after[j] {
				if v != before[j][k] {
					t.Errorf("registering on app %d added %d handler(s) to app %d's %s", i, v-before[j][k], j, k)
				}
			}
		}
	}

	more := make([]*core.BaseApp, o.Apps)
	errs := make([]error, len(more))
	var wg sync.WaitGroup
	for i := range more {
		more[i] = newApp(t, o)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = p.Register(more[i], o.Layers)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("concurrent Register on app %d: %v", i, err)
		}
	}
	if t.Failed() {
		return
	}
	for i, app := range append(apps, more...) {
		if err := run(app); err != nil {
			t.Errorf("app %d: %v", i, err)
		}
	}
	for _, app := range append(apps, more...) {
		_ = stop(app)
	}
}

// CheckMCPTools: every MCP tool the extension adds (to the server [Options.Setup] put on the app) is named
// `<name>_<verb>_<noun>`: the extension's name (with "-" as "_"), then at least two lowercase words joined by "_". Each
// has a description, and an input schema, when it has one, is a JSON Schema object. The `pb_` prefix is
// PocketBase's own.
func CheckMCPTools(t TB, p extkit.Extension, o Options) {
	app := newApp(t, o)
	before, ok := extkit.Tools(app)
	if !ok {
		t.Fatalf("Options.Setup put no MCP server on the app (ext/mcp's Register does)")
	}
	if err := start(app, p, o.Layers); err != nil {
		t.Fatal(err)
	}
	defer stop(app)
	after, _ := extkit.Tools(app)

	had := map[string]bool{}
	for _, tool := range before {
		had[tool.Name] = true
	}
	for _, tool := range extkit.ProvidedTools(app) { // a contract's tools are named for the contract (restore_points)
		had[tool.Name] = true
	}
	prefix := strings.ReplaceAll(p.Name(), "-", "_") + "_"
	for _, tool := range after {
		if had[tool.Name] {
			continue
		}
		rest, found := strings.CutPrefix(tool.Name, prefix)
		if !found || !toolWords.MatchString(rest) {
			t.Errorf("MCP tool %q: name it %s<verb>_<noun> (lowercase words joined by _, e.g. %slist_items)", tool.Name, prefix, prefix)
		}
		if strings.TrimSpace(tool.Description) == "" {
			t.Errorf("MCP tool %q has no description; models pick tools by it", tool.Name)
		}
		if tool.InputSchema != nil && tool.InputSchema["type"] != "object" {
			t.Errorf("MCP tool %q: input schema must be a JSON Schema object ({\"type\": \"object\", ...})", tool.Name)
		}
	}
}

// toolWords is the part of a tool name after the extension prefix: a verb and a noun at least.
var toolWords = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)+$`)

func allHookCounts(apps []*core.BaseApp) []map[string]int {
	out := make([]map[string]int, len(apps))
	for i, a := range apps {
		out[i] = hookCounts(a)
	}
	return out
}

var appSeq atomic.Int64

// newApp is a fresh app with its own host id ([extkit.AppID]), as an isolate runtime gives each instance.
func newApp(t TB, o Options) *core.BaseApp {
	t.Helper()
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir(), EncryptionEnv: "", QueryTimeout: 10 * time.Second})
	extkit.SetAppID(app, fmt.Sprintf("app%d", appSeq.Add(1)))
	if o.Setup != nil {
		if err := o.Setup(app); err != nil {
			t.Fatalf("Options.Setup: %v", err)
		}
	}
	for _, d := range o.Deps {
		extkit.AppScope(app).Registering(d.Name())
		if err := d.Register(app, nil); err != nil {
			t.Fatalf("Options.Deps: %s: %v", d.Name(), err)
		}
	}
	deps.Store(app, o.Deps)
	return app
}

// deps remembers each test app's Options.Deps, so start can seal the app's contracts with them.
var deps sync.Map // *core.BaseApp → []extkit.Extension

// start registers p (if any) and runs the app up to serving.
func start(app *core.BaseApp, p extkit.Extension, layers extkit.Config) error {
	on, _ := deps.Load(app)
	plan := &extkit.Plan{}
	if d, ok := on.([]extkit.Extension); ok {
		plan.On = append(plan.On, d...)
	}
	if p != nil {
		extkit.AppScope(app).Registering(p.Name())
		if err := p.Register(app, layers); err != nil {
			return fmt.Errorf("Register: %w", err)
		}
		plan.On = append(plan.On, p)
	}
	// seal the app's contracts as the standard set does, which also checks the extension provided what it declares
	if _, err := extkit.Finish(app, plan, nil); err != nil {
		return err
	}
	return run(app)
}

// CheckRoutes: every route the extension adds is a contract's, under /api/_c/ (mounted by extkit); or /.well-known/
// (what a standard fixes at the root), /mcp, or the dashboard's /_/extensions/ (its admin screens), unless
// Options.AllowRoutes lists it with a reason. Extensions have no routes of their own: their API is contracts.
func CheckRoutes(t TB, p extkit.Extension, o Options) {
	without := newApp(t, o)
	if err := start(without, nil, nil); err != nil {
		t.Fatal(err)
	}
	before := routesOf(without)
	_ = stop(without)

	app := newApp(t, o)
	if err := start(app, p, o.Layers); err != nil {
		t.Fatal(err)
	}
	defer stop(app)
	for route := range routesOf(app) {
		if before[route] {
			continue
		}
		path := route
		if i := strings.IndexByte(route, ' '); i >= 0 {
			path = route[i+1:]
		}
		switch {
		case strings.HasPrefix(path, extkit.ContractPrefix), strings.HasPrefix(path, "/.well-known/"), path == "/mcp",
			strings.HasPrefix(path, "/_/extensions/"):
			continue
		}
		if _, ok := o.AllowRoutes[path]; ok {
			continue
		}
		t.Errorf("route %q: an extension's API is contracts, under %s<id>/ (ext/contracts); or list it in AllowRoutes with a reason", route, extkit.ContractPrefix)
	}
}

var lastRouters sync.Map // *core.BaseApp → the router its last run built

// routesOf lists the routes ("METHOD /path") app's last run registered.
func routesOf(app *core.BaseApp) map[string]bool {
	out := map[string]bool{}
	r, ok := lastRouters.Load(app)
	if !ok {
		return out
	}
	walkRoutes(reflect.ValueOf(r.(*router.Router[*core.RequestEvent]).RouterGroup), "", out)
	return out
}

// walkRoutes reads the router's tree (its children aren't exported; this is test code, so reflection is fine).
func walkRoutes(group reflect.Value, prefix string, out map[string]bool) {
	g := reflect.Indirect(group)
	prefix += g.FieldByName("Prefix").String()
	children := g.FieldByName("children")
	for i := 0; i < children.Len(); i++ {
		c := children.Index(i).Elem()
		v := reflect.Indirect(c)
		if v.FieldByName("children").IsValid() {
			walkRoutes(c, prefix, out)
			continue
		}
		method := v.FieldByName("Method").String()
		out[strings.TrimSpace(method+" "+prefix+v.FieldByName("Path").String())] = true
	}
}

// run bootstraps the app, applies every migration and triggers OnServe with a real router (no listener).
func run(app *core.BaseApp) error {
	if err := app.Bootstrap(); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	if err := app.RunAllMigrations(); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	r, err := apis.NewRouter(app)
	if err != nil {
		return fmt.Errorf("router: %w", err)
	}
	lastRouters.Store(app, r)
	se := &core.ServeEvent{App: app, Router: r, Server: &http.Server{Addr: "127.0.0.1:0"}}
	if err := app.OnServe().Trigger(se, func(e *core.ServeEvent) error {
		_, err := e.Router.BuildMux()
		return err
	}); err != nil {
		return fmt.Errorf("serve hooks: %w", err)
	}
	return nil
}

func stop(app *core.BaseApp) error {
	err := app.OnTerminate().Trigger(&core.TerminateEvent{App: app}, func(e *core.TerminateEvent) error {
		return e.App.ResetBootstrapState()
	})
	return err
}

// snapshot is every collection's definition, minus timestamps and index quoting.
func snapshot(t TB, app core.App) map[string]string {
	t.Helper()
	cols, err := app.FindAllCollections()
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]string, len(cols))
	for _, c := range cols {
		var m map[string]any
		b, _ := json.Marshal(c)
		_ = json.Unmarshal(b, &m)
		delete(m, "created")
		delete(m, "updated")
		if idx, ok := m["indexes"].([]any); ok { // PocketBase re-quotes index SQL when it renames a table
			for i, v := range idx {
				if sql, ok := v.(string); ok {
					idx[i] = strings.NewReplacer("`", "", " ", "", "\n", "", "\t", "").Replace(sql)
				}
			}
		}
		b, _ = json.Marshal(m)
		out[c.Name] = string(b)
	}
	return out
}

func hookCounts(app core.App) map[string]int {
	return map[string]int{
		"OnBootstrap":              app.OnBootstrap().Length(),
		"OnServe":                  app.OnServe().Length(),
		"OnTerminate":              app.OnTerminate().Length(),
		"OnBackupCreate":           app.OnBackupCreate().Length(),
		"OnBackupRestore":          app.OnBackupRestore().Length(),
		"OnMailerSend":             app.OnMailerSend().Length(),
		"OnRealtimeConnectRequest": app.OnRealtimeConnectRequest().Length(),
		"OnSettingsReload":         app.OnSettingsReload().Length(),
	}
}

// goroutines returns the stacks of all goroutines, keyed by their first function frame.
func goroutines() map[string]int {
	out := map[string]int{}
	for _, g := range stacks() {
		out[topFrame(g)]++
	}
	return out
}

// waitGoroutines waits up to d for the goroutine set to return to before (plus slack) and returns the stacks of
// any that stayed.
func waitGoroutines(before map[string]int, slack int, d time.Duration) []string {
	deadline := time.Now().Add(d)
	for {
		var extra []string
		seen := map[string]int{}
		for _, g := range stacks() {
			top := topFrame(g)
			seen[top]++
			if seen[top] > before[top] && !ignorable(g) {
				extra = append(extra, g)
			}
		}
		if len(extra) <= slack {
			return nil
		}
		if time.Now().After(deadline) {
			return extra
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func stacks() []string {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	return strings.Split(strings.TrimSpace(string(buf)), "\n\n")
}

func topFrame(stack string) string {
	lines := strings.Split(stack, "\n")
	if len(lines) < 2 {
		return stack
	}
	return strings.TrimSpace(lines[1])
}

// ignorable goroutines belong to the test runner or the Go runtime, not to the extension.
func ignorable(stack string) bool {
	top := topFrame(stack)
	for _, s := range []string{"testing.", "runtime.", "os/signal.", "conformance.stacks"} {
		if strings.HasPrefix(top, s) {
			return true
		}
	}
	// the goroutine running the check itself
	return strings.Contains(stack, "conformance.waitGoroutines")
}
