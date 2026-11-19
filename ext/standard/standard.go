// Package standard is PocketOne's standard set: the extensions every PocketOne app has (a standalone app or an
// instance on a multisite host, whatever its runtime), with named members a host configures and extras it adds.
//
//	set := standard.New()
//	set.PITR = pitr.New(pitr.Config{Retention: 7 * 24 * time.Hour}) // reconfigure a member
//	set.Add(mailtrap.New())                                         // the host's own extensions
//	app := set.NewApp(pocketbase.Config{}, jsvm.Config{})
//
// Members can be reconfigured, never removed: a single app turns one off with `enabled: false` in its pocketone.yaml
// section, an instance on a multisite host in its `extensions.<name>`, and the startup lines say so. Extensions find each other through contracts (ext/contracts), resolved per app.
package standard

import (
	"context"
	"fmt"
	clicmdv1 "github.com/pockethost/pocketone/ext/contracts/cli/commands/v1"
	"log/slog"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
	"github.com/pockethost/pocketone/ext/agent"
	"github.com/pockethost/pocketone/ext/backups"
	"github.com/pockethost/pocketone/ext/bulkmail"
	"github.com/pockethost/pocketone/ext/cache"
	"github.com/pockethost/pocketone/ext/deploy"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/js"
	"github.com/pockethost/pocketone/ext/jsvm"
	"github.com/pockethost/pocketone/ext/mcp"
	"github.com/pockethost/pocketone/ext/pages"
	"github.com/pockethost/pocketone/ext/pitr"
	"github.com/pockethost/pocketone/ext/remotes"
	s3fallback "github.com/pockethost/pocketone/ext/s3-fallback"
	"github.com/pockethost/pocketone/ext/s3redirect"
	"github.com/pockethost/pocketone/ext/settings"
	"github.com/pockethost/pocketone/ext/templates"
	"github.com/pockethost/pocketone/ext/tokens"
	"github.com/pockethost/pocketone/ext/wasm"
)

// Set is the standard members, then the host's extras, registered on every app in that order (contracts reorder them
// so providers come first).
type Set struct {
	Settings   *settings.Extension
	Templates  *templates.Extension
	Cache      *cache.Extension
	Tokens     *tokens.Extension
	Remotes    *remotes.Extension
	MCP        *mcp.Extension
	Pages      *pages.Extension
	S3Fallback *s3fallback.Extension
	S3Redirect *s3redirect.Extension
	PITR       *pitr.Extension
	Backups    *backups.Extension
	BulkMail   *bulkmail.Extension
	Deploy     *deploy.Extension
	WASM       *wasm.Extension
	Agent      *agent.Extension

	// Providers is the `providers:` section of pocketone.yaml: which extension provides a contract when several do.
	Providers extkit.Choices

	extras []extkit.Extension
}

var _ extkit.AppSet = (*Set)(nil)

// New returns every standard member with its defaults.
func New() *Set {
	return &Set{
		Settings:   settings.New(),
		Templates:  templates.New(templates.Config{}),
		Cache:      cache.New(cache.Config{}),
		Tokens:     tokens.New(tokens.Config{}),
		Remotes:    remotes.New(remotes.Config{}),
		MCP:        mcp.New(mcp.Config{}),
		Pages:      pages.New(pages.Config{}),
		S3Fallback: s3fallback.New(s3fallback.Config{}),
		S3Redirect: s3redirect.New(s3redirect.Config{}),
		PITR:       pitr.New(pitr.Config{}),
		Backups:    backups.New(),
		BulkMail:   bulkmail.New(bulkmail.Config{}),
		Deploy:     deploy.NewExtension(deploy.Config{}),
		WASM:       wasm.New(wasm.Config{}),
		Agent:      agent.New(),
	}
}

// Add registers the host's own extensions on every app, after the standard members, under the same rules.
func (s *Set) Add(e ...extkit.Extension) *Set {
	s.extras = append(s.extras, e...)
	return s
}

// Extensions lists the members then the extras.
func (s *Set) Extensions() []extkit.Extension {
	out := make([]extkit.Extension, 0, 15+len(s.extras))
	for _, m := range s.members() {
		if m.ext != nil {
			out = append(out, m.ext)
		}
	}
	return append(out, s.extras...)
}

type member struct {
	field string
	ext   extkit.Extension
}

func (s *Set) members() []member {
	m := []member{
		{"Settings", s.Settings}, {"Templates", s.Templates}, {"Cache", s.Cache}, {"Tokens", s.Tokens}, {"Remotes", s.Remotes}, {"MCP", s.MCP}, {"Pages", s.Pages},
		{"S3Fallback", s.S3Fallback}, {"S3Redirect", s.S3Redirect}, {"PITR", s.PITR}, {"Backups", s.Backups}, {"BulkMail", s.BulkMail}, {"Deploy", s.Deploy}, {"WASM", s.WASM}, {"Agent", s.Agent},
	}
	for i := range m { // a typed nil pointer in an interface isn't nil
		if isNil(m[i].ext) {
			m[i].ext = nil
		}
	}
	return m
}

func isNil(e extkit.Extension) bool {
	switch v := e.(type) {
	case *settings.Extension:
		return v == nil
	case *templates.Extension:
		return v == nil
	case *cache.Extension:
		return v == nil
	case *tokens.Extension:
		return v == nil
	case *remotes.Extension:
		return v == nil
	case *mcp.Extension:
		return v == nil
	case *pages.Extension:
		return v == nil
	case *s3fallback.Extension:
		return v == nil
	case *s3redirect.Extension:
		return v == nil
	case *pitr.Extension:
		return v == nil
	case *backups.Extension:
		return v == nil
	case *bulkmail.Extension:
		return v == nil
	case *deploy.Extension:
		return v == nil
	case *wasm.Extension:
		return v == nil
	case *agent.Extension:
		return v == nil
	}
	return e == nil
}

func (s *Set) check() error {
	for _, m := range s.members() {
		if m.ext == nil {
			key := strings.ToLower(m.field)
			if m.field == "S3Fallback" {
				key = s3fallback.Name // its section is `storage`
			}
			return fmt.Errorf("standard: Set.%s is nil; members can be reconfigured, not removed (an app turns one off with `%s: {enabled: false}`)", m.field, key)
		}
	}
	seen := map[string]bool{}
	for _, e := range s.Extensions() {
		if seen[e.Name()] {
			return fmt.Errorf("standard: two extensions are named %q (a standard member is already in the set; reconfigure it instead of adding another)", e.Name())
		}
		seen[e.Name()] = true
	}
	return nil
}

const planKey = "standard.plan"

// Prepare implements [extkit.AppSet]: extensions whose settings say `enabled: false` are off, the rest are resolved
// through their contracts and registered.
func (s *Set) Prepare(app core.App, layers func(name string) extkit.Config) (*extkit.Plan, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if layers == nil {
		layers = func(string) extkit.Config { return nil }
	}
	var on []extkit.Extension
	var offLines []string
	for _, e := range s.Extensions() {
		if !layers(e.Name()).Enabled() {
			offLines = append(offLines, e.Name()+" off: enabled: false")
			continue
		}
		on = append(on, e)
	}
	plan, err := extkit.Prepare(app, on, layers, s.Providers)
	if err != nil {
		return nil, err
	}
	plan.Lines = append(offLines, plan.Lines...)
	app.Store().Set(planKey, plan)
	return plan, nil
}

// JSExtensions implements [extkit.AppSet]: JS contracts, plus every extension on for app that declares its JS API's
// types (js.Typer), for pb_data/types.d.ts.
func (s *Set) JSExtensions(app core.App) []any {
	out := []any{extkit.JSContracts(app)}
	if plan, _ := app.Store().Get(planKey).(*extkit.Plan); plan != nil {
		for _, e := range plan.On {
			if t, ok := e.(js.Typer); ok {
				out = append(out, typesOnly{t}) // only its types: the set binds its JS API itself (BindJS)
			}
		}
	}
	return out
}

// BindJS implements [extkit.AppSet]: the JS API of every extension that runs on app.
func (s *Set) BindJS(app core.App, rt js.Runtime) {
	plan, _ := app.Store().Get(planKey).(*extkit.Plan)
	if plan == nil {
		return
	}
	for _, e := range plan.On {
		if b, ok := e.(extkit.JSBinder); ok {
			b.BindJS(app, rt)
		}
	}
}

// Finish implements [extkit.AppSet]: it seals app's contracts and returns the startup lines. The host decides where
// they go: a standalone app logs them (Register), Pocker writes them to the instance's own log.
func (s *Set) Finish(app core.App, plan *extkit.Plan) ([]string, error) {
	lines, err := extkit.Finish(app, plan, s.Providers)
	if err != nil {
		return nil, err
	}
	on := make([]string, 0, len(plan.On))
	for _, e := range plan.On {
		on = append(on, e.Name())
	}
	return append([]string{"standard: on: " + strings.Join(on, ", ")}, lines...), nil
}

// NewApp is a standalone PocketOne app: the set registered, pb_hooks loaded through jsvm with the set's JS APIs, the
// contracts sealed. It panics on a setup error, like PocketBase's MustRegister helpers.
func (s *Set) NewApp(cfg pocketbase.Config, jsCfg jsvm.Config) *pocketbase.PocketBase {
	app := pocketbase.NewWithConfig(cfg)
	if err := s.Register(app, jsCfg); err != nil {
		panic(err)
	}
	return app
}

// Register sets the set up on an app you built yourself: Prepare, jsvm with the set's JS extensions and binds
// (chained before jsCfg.OnInit), Finish.
func (s *Set) Register(app core.App, jsCfg jsvm.Config) error {
	return s.RegisterWith(app, func(app core.App) error {
		jsCfg.Extensions = append(jsCfg.Extensions, s.JSExtensions(app)...)
		userInit := jsCfg.OnInit
		jsCfg.OnInit = func(rt js.Runtime) {
			s.BindJS(app, rt)
			if userInit != nil {
				userInit(rt)
			}
		}
		return jsvm.Register(app, jsCfg)
	})
}

// RegisterWith is Register with another JS engine: loadJS registers it on app between the extensions and the seal.
// The pocketone binary passes PocketBase's own plugins/jsvm here in its goja compatibility mode; then the set's JS
// APIs ($mcp, $tokens, $wasm, pages, JS contracts) aren't bound.
func (s *Set) RegisterWith(app core.App, loadJS func(app core.App) error) error {
	plan, err := s.Prepare(app, ConfiguredSwitches) // pocketone.yaml's `<member>: {enabled: false}`
	if err != nil {
		return err
	}
	if err := loadJS(app); err != nil {
		return err
	}
	lines, err := s.Finish(app, plan)
	if err != nil {
		return err
	}
	// printed to the terminal when the app serves, so `--version`, `help` and other commands print only their own
	// output (slog's default logger: after bootstrap, app.Logger() writes to the logs database instead)
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(e *core.ServeEvent) error {
			for _, l := range lines {
				slog.Info(l)
			}
			return e.Next()
		},
	})
	// the command tree: every contract operation as `<binary> remote ...`, and the commands extensions provide
	// (cli.commands/v1)
	if pb, ok := app.(*pocketbase.PocketBase); ok {
		clicmdv1.Mount(app, pb.RootCmd)
	}
	// a single app owns the process: close shared resources (pitr's stores) after its own terminate has run
	app.OnTerminate().Bind(&hook.Handler[*core.TerminateEvent]{
		Priority: -1000,
		Func: func(e *core.TerminateEvent) error {
			err := e.Next()
			if !e.IsRestart {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				for _, x := range plan.On {
					if c, ok := x.(extkit.Closer); ok {
						_ = c.Close(ctx)
					}
				}
			}
			return err
		},
	})
	return nil
}

// ProvidersSection is pocketone.yaml's `providers:` section: for a contract several extensions provide, which one
// (or, for one that allows many, which ones in order), or `off`.
//
//	providers:
//	  mail.fallback/v1: mailrelay
//	  runtime/v1: [pocker, docker]
var ProvidersSection = extkit.NewSection("providers", map[string]any{}, "Which extension provides a contract when several do.")

// ConfiguredProviders reads the `providers:` section as choices for [Set.Providers].
func ConfiguredProviders() extkit.Choices {
	m, _ := ProvidersSection.Get()
	out := extkit.Choices{}
	for k, v := range m {
		if inner, ok := v.(map[string]any); ok { // keyed contracts: key → extension
			picks := map[string]string{}
			for key, name := range inner {
				picks[key] = fmt.Sprint(name)
			}
			out[extkit.ContractID(k)] = picks
			continue
		}
		out[extkit.ContractID(k)] = v
	}
	return out
}

// typesOnly passes an extension's TypeScript declarations to jsvm without its other methods.
type typesOnly struct{ t js.Typer }

func (t typesOnly) TypeScript() string { return t.t.TypeScript() }
