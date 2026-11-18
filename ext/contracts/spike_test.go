package contracts_test

// The spike's end-to-end checks of the extension-contracts design (pocketbrain plans/extension-contracts.md), with
// toy extensions standing in for deploy, issuera, issuerb, a quota extension, a wrapper and a host.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	examplev1 "github.com/pockethost/pocketone/ext/contracts/internal/example/v1"
	tokensv1 "github.com/pockethost/pocketone/ext/contracts/tokens/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// ---- toy extensions -----------------------------------------------------------------------------------------

// ext is a configurable toy extension.
type ext struct {
	name     string
	decl     extkit.Contracts
	register func(app core.App) error
}

func (e *ext) Name() string                { return e.name }
func (e *ext) Contracts() extkit.Contracts { return e.decl }
func (e *ext) Register(app core.App, _ extkit.Config) error {
	if e.register == nil {
		return nil
	}
	return e.register(app)
}

// folderDeploy: the standard deploy extension; provides deploy/v1 as a default.
type folderDeployer struct{ by string }

func (d folderDeployer) Manifest(context.Context, extkit.Auth) (examplev1.Manifest, error) {
	return examplev1.Manifest{Files: []examplev1.File{{Path: "pb_hooks/main.pb.js", Size: 10}}}, nil
}
func (d folderDeployer) Upload(_ context.Context, _ extkit.Auth, in examplev1.UploadInput) (examplev1.UploadResult, error) {
	return examplev1.UploadResult{Written: len(in.Files), By: d.by}, nil
}

func deployExt() *ext {
	return &ext{name: "deploy",
		decl: extkit.Contracts{Implements: extkit.IDs("example/v1"), Defaults: extkit.IDs("example/v1")},
		register: func(app core.App) error {
			examplev1.Contract.ProvideDefault(app, folderDeployer{by: "deploy"})
			return nil
		}}
}

func fastDeployExt() *ext {
	return &ext{name: "fastdeploy",
		decl: extkit.Contracts{Implements: extkit.IDs("example/v1")},
		register: func(app core.App) error {
			examplev1.Contract.Provide(app, folderDeployer{by: "fastdeploy"})
			return nil
		}}
}

// quota: works with deploy/v1; refuses uploads of more than max files, on every surface.
func quotaExt(max int) *ext {
	return &ext{name: "quota",
		decl: extkit.Contracts{WorksWith: extkit.IDs("example/v1")},
		register: func(app core.App) error {
			examplev1.OnUpload(app, func(e *extkit.CallEvent, in *examplev1.UploadInput) error {
				if len(in.Files) > max {
					return examplev1.ErrQuota
				}
				return e.Next()
			})
			return nil
		}}
}

// audit: wraps deploy/v1, marking results.
type tagged struct {
	examplev1.Deployer
	tag string
}

func (t tagged) Upload(ctx context.Context, a extkit.Auth, in examplev1.UploadInput) (examplev1.UploadResult, error) {
	r, err := t.Deployer.Upload(ctx, a, in)
	r.By = t.tag + "(" + r.By + ")"
	return r, err
}

func wrapExt(name string) *ext {
	return &ext{name: name,
		decl: extkit.Contracts{Wraps: extkit.IDs("example/v1")},
		register: func(app core.App) error {
			examplev1.Contract.Wrap(app, func(inner examplev1.Deployer) examplev1.Deployer { return tagged{inner, name} })
			return nil
		}}
}

type sender struct{ name string }

func (s sender) Verify(context.Context, string) (tokensv1.Grant, error) { return tokensv1.Grant{}, nil }
func (s sender) RegisterScope(tokensv1.Scope) error                     { return nil }
func (s sender) Scopes() []tokensv1.Scope                               { return nil }
func (s sender) Reaches([]string, string, string) bool                  { return true }

func mailExt(name string, isDefault bool) *ext {
	return &ext{name: name,
		decl: extkit.Contracts{Implements: extkit.IDs("tokens/v1")},
		register: func(app core.App) error {
			if isDefault {
				tokensv1.Contract.ProvideDefault(app, sender{name})
			} else {
				tokensv1.Contract.Provide(app, sender{name})
			}
			return nil
		}}
}

func needsMailExt() *ext {
	return &ext{name: "needer", decl: extkit.Contracts{Requires: extkit.IDs("tokens/v1")}}
}

func newApp(t *testing.T) *tests.TestApp {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	return app
}

func setup(t *testing.T, app core.App, choices extkit.Choices, exts ...extkit.Extension) []string {
	t.Helper()
	lines, err := extkit.Setup(app, exts, nil, choices)
	if err != nil {
		t.Fatal(err)
	}
	return lines
}

func contains(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

func superuserAuth() extkit.Auth { return extkit.Auth{Kind: "superuser", Via: "go"} }

// ---- resolution ---------------------------------------------------------------------------------------------

func TestDefaultStepsAside(t *testing.T) {
	app := newApp(t)
	lines := setup(t, app, nil, deployExt(), fastDeployExt())
	r, err := examplev1.Contract.Must(app).Upload(context.Background(), superuserAuth(), examplev1.UploadInput{})
	if err != nil || r.By != "fastdeploy" {
		t.Fatalf("got %+v %v, want fastdeploy", r, err)
	}
	if !contains(lines, "example/v1 ← fastdeploy (default deploy stepped aside)") {
		t.Fatalf("startup lines: %q", lines)
	}
}

func TestOneConflictNeedsAChoice(t *testing.T) {
	app := newApp(t)
	_, err := extkit.Setup(app, []extkit.Extension{mailExt("issuera", false), mailExt("issuerb", false)}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "conflict: tokens/v1 has 2 providers (issuera, issuerb)") {
		t.Fatalf("err = %v", err)
	}

	app2 := newApp(t)
	lines := setup(t, app2, extkit.Choices{"tokens/v1": "issuerb"}, mailExt("issuera", false), mailExt("issuerb", false))
	if s := tokensv1.Contract.Must(app2).(sender); s.name != "issuerb" {
		t.Fatalf("got %s", s.name)
	}
	if !contains(lines, "tokens/v1 ← issuerb (providers:)") {
		t.Fatalf("lines %q", lines)
	}
}

func TestChoiceTypos(t *testing.T) {
	app := newApp(t)
	_, err := extkit.Setup(app, []extkit.Extension{mailExt("issuera", true)}, nil, extkit.Choices{"mail.fallbak/v1": "issuera"})
	if err == nil || !strings.Contains(err.Error(), "mail.fallbak/v1: no extension here provides or uses it") {
		t.Fatalf("err = %v", err)
	}
	app2 := newApp(t)
	_, err = extkit.Setup(app2, []extkit.Extension{mailExt("issuera", true)}, nil, extkit.Choices{"tokens/v1": "issuerb"})
	if err == nil || !strings.Contains(err.Error(), "issuerb doesn't provide it here") {
		t.Fatalf("err = %v", err)
	}
}

func TestRequiresTurnsOff(t *testing.T) {
	app := newApp(t)
	lines := setup(t, app, extkit.Choices{"tokens/v1": "off"}, mailExt("issuera", true), needsMailExt())
	if !contains(lines, "needer off: needs tokens/v1, which providers: turns off") {
		t.Fatalf("lines %q", lines)
	}
	if _, ok := tokensv1.Contract.Of(app); ok {
		t.Fatal("mail.fallback/v1 should have no provider")
	}

	app2 := newApp(t)
	lines = setup(t, app2, nil, needsMailExt())
	if !contains(lines, "needer off: needs tokens/v1, which nothing provides") {
		t.Fatalf("lines %q", lines)
	}
}

func TestLookupBeforeSealPanics(t *testing.T) {
	app := newApp(t)
	early := &ext{name: "early", register: func(app core.App) error {
		_, _ = examplev1.Contract.Of(app)
		return nil
	}}
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(fmt.Sprint(r), "before the seal") {
			t.Fatalf("recover = %v", r)
		}
	}()
	_, _ = extkit.Setup(app, []extkit.Extension{deployExt(), early}, nil, nil)
}

func TestWrappersStackInRegistrationOrder(t *testing.T) {
	app := newApp(t)
	lines := setup(t, app, nil, wrapExt("outer"), deployExt(), wrapExt("inner"))
	// resolver puts deploy first (both wrappers depend on it), then outer, inner in registration order
	r, err := examplev1.Contract.Must(app).Upload(context.Background(), superuserAuth(), examplev1.UploadInput{})
	if err != nil || r.By != "inner(outer(deploy))" {
		t.Fatalf("got %+v %v", r, err)
	}
	if !contains(lines, "example/v1 ← inner(outer(deploy))") {
		t.Fatalf("lines %q", lines)
	}
}

func TestImplementsIsChecked(t *testing.T) {
	app := newApp(t)
	liar := &ext{name: "liar", decl: extkit.Contracts{Implements: extkit.IDs("example/v1")}}
	_, err := extkit.Setup(app, []extkit.Extension{liar}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "liar declares it implements example/v1 but didn't provide it") {
		t.Fatalf("err = %v", err)
	}
}

// ---- every surface goes through the same scope check and hooks ---------------------------------------------

func serve(t *testing.T, app core.App) *httptest.Server {
	t.Helper()
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	// Finish bound the contract's routes to OnServe; trigger it on our router
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

func superuserToken(t *testing.T, app core.App) string {
	t.Helper()
	su, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := su.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestSameHooksOnHTTPMCPAndGo(t *testing.T) {
	app := newApp(t)
	setup(t, app, nil, deployExt(), quotaExt(2))
	srv := serve(t, app)
	ctx := context.Background()
	two := examplev1.UploadInput{Files: make([]examplev1.File, 2)}
	three := examplev1.UploadInput{Files: make([]examplev1.File, 3)}

	// HTTP through the contract's own client (what the CLI uses)
	c := examplev1.NewClient(srv.URL, superuserToken(t, app), nil)
	if r, err := c.Upload(ctx, extkit.Auth{}, two); err != nil || r.Written != 2 {
		t.Fatalf("http upload: %+v %v", r, err)
	}
	_, err := c.Upload(ctx, extkit.Auth{}, three)
	var ce *extkit.Error
	if !errors.As(err, &ce) || ce.Code != "quota" || ce.Status != 413 {
		t.Fatalf("http over quota: %v", err)
	}

	// anonymous over HTTP: 401 before any hook
	_, err = examplev1.NewClient(srv.URL, "", nil).Manifest(ctx, extkit.Auth{})
	if !errors.As(err, &ce) || ce.Status != 401 {
		t.Fatalf("anonymous: %v", err)
	}

	// MCP tool: calls the contract directly, still hooked
	var upload extkit.Tool
	for _, tl := range examplev1.W.Tools(app) {
		if tl.Name == "example_upload" {
			upload = tl
		}
	}
	if _, err := upload.Call(ctx, extkit.Auth{Kind: "token", Collection: "_superusers", Scopes: []string{"example"}}, []byte(`{"files":[{},{},{}]}`)); !errors.Is(err, examplev1.ErrQuota) {
		t.Fatalf("mcp over quota: %v", err)
	}
	// a signed-in app user who isn't a superuser can't call a contract operation, whatever its scopes
	if _, err := upload.Call(ctx, extkit.Auth{Kind: "record", Collection: "users"}, []byte(`{}`)); !errors.Is(err, extkit.ErrNotSuperuser) {
		t.Fatalf("record user: %v", err)
	}
	if _, err := upload.Call(ctx, extkit.Auth{Kind: "token", Collection: "users"}, []byte(`{}`)); !errors.Is(err, extkit.ErrNotSuperuser) {
		t.Fatalf("a user's token: %v", err)
	}
	// a token without the deploy scope can't call it, whatever the surface
	if _, err := upload.Call(ctx, extkit.Auth{Kind: "token", Collection: "_superusers", Scopes: []string{"mcp"}}, []byte(`{}`)); !errors.Is(err, extkit.ErrForbidden) {
		t.Fatalf("mcp without scope: %v", err)
	}

	// Go caller in the same process
	if _, err := examplev1.Contract.Must(app).Upload(ctx, superuserAuth(), three); !errors.Is(err, examplev1.ErrQuota) {
		t.Fatalf("go over quota: %v", err)
	}
}

// ---- host-provided: the edge answers deploy for its instances ---------------------------------------------

type edgeDeployer struct{ instance string }

func (d edgeDeployer) Manifest(context.Context, extkit.Auth) (examplev1.Manifest, error) {
	return examplev1.Manifest{Files: []examplev1.File{{Path: d.instance + "/pb_public/index.html"}}}, nil
}
func (d edgeDeployer) Upload(_ context.Context, _ extkit.Auth, in examplev1.UploadInput) (examplev1.UploadResult, error) {
	return examplev1.UploadResult{Written: len(in.Files), By: "edge:" + d.instance}, nil
}

func TestHostProvidesDeployForItsInstances(t *testing.T) {
	host := extkit.NewScope("multisite")
	host.Registering("phdeploy")
	examplev1.Contract.ProvideForInstances(host, func(inst string) examplev1.Deployer { return edgeDeployer{inst} })
	ctx := context.Background()

	// A Docker instance runs stock PocketBase: nothing of ours inside. The host answers /api/_c/example/v1 on its
	// hostname in front of the container; everything else reaches the container.
	stock := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(418) // "the stock container"
	})
	instanceOf := func(r *http.Request) (string, bool) {
		return strings.SplitN(r.Host, ".", 2)[0], true
	}
	hostAuth := func(r *http.Request) extkit.Auth {
		if r.Header.Get("Authorization") == "ph_pat_ok" {
			return extkit.Auth{Kind: "token", Collection: "_superusers", Scopes: []string{"example"}}
		}
		return extkit.Auth{}
	}
	edge := httptest.NewServer(examplev1.W.HostHandler(host, instanceOf, hostAuth, stock))
	t.Cleanup(edge.Close)
	viaHost := func(hostname string) *http.Client {
		return &http.Client{Transport: hostRewrite{hostname}}
	}

	c := &edgeClient{edge.URL, viaHost("dock1.pockethost.io")}
	if r, err := c.upload(ctx, "ph_pat_ok", 2); err != nil || r.By != "edge:dock1" {
		t.Fatalf("docker instance deploy: %+v %v", r, err)
	}
	if _, err := c.upload(ctx, "", 1); status(err) != 401 {
		t.Fatalf("docker anonymous: %v", err)
	}
	// host-side hooks (owned by the host scope) apply to calls the host answers
	examplev1.OnUpload(host, func(e *extkit.CallEvent, in *examplev1.UploadInput) error {
		if len(in.Files) > 5 {
			return examplev1.ErrQuota
		}
		return e.Next()
	})
	if _, err := c.upload(ctx, "ph_pat_ok", 6); status(err) != 413 {
		t.Fatalf("host hook: %v", err)
	}
	res, _ := viaHost("dock1.pockethost.io").Get(edge.URL + "/api/health")
	if res.StatusCode != 418 {
		t.Fatalf("non-contract path should reach the container, got %d", res.StatusCode)
	}

	// An in-process (Pocker) instance is a PocketOne app: its own deploy extension steps aside, and in-app callers get the host's
	// provider for this instance, still through the app's own hooks.
	app := newApp(t)
	extkit.AttachHost(app, host, "pock1")
	lines := setup(t, app, nil, deployExt(), quotaExt(1))
	if !contains(lines, "standard[pock1]: example/v1 ← host (phdeploy); stepped aside in the app: deploy") {
		t.Fatalf("lines %q", lines)
	}
	r, err := examplev1.Contract.Must(app).Upload(ctx, superuserAuth(), examplev1.UploadInput{Files: make([]examplev1.File, 1)})
	if err != nil || r.By != "edge:pock1" {
		t.Fatalf("pocker in-app: %+v %v", r, err)
	}
	if _, err := examplev1.Contract.Must(app).Upload(ctx, superuserAuth(), examplev1.UploadInput{Files: make([]examplev1.File, 2)}); !errors.Is(err, examplev1.ErrQuota) {
		t.Fatalf("app hook on in-app call: %v", err)
	}
}

type hostRewrite struct{ host string }

func (h hostRewrite) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Host = h.host
	return http.DefaultTransport.RoundTrip(r)
}

type edgeClient struct {
	url string
	hc  *http.Client
}

func (c *edgeClient) upload(ctx context.Context, token string, n int) (examplev1.UploadResult, error) {
	var r examplev1.UploadResult
	caller := examplev1.W.Client(c.url, token, c.hc)
	return r, caller.Call(ctx, "POST", "upload", examplev1.UploadInput{Files: make([]examplev1.File, n)}, &r)
}

func status(err error) int {
	var ce *extkit.Error
	if errors.As(err, &ce) {
		return ce.Status
	}
	return 0
}

// ---- keyed: MCP tools by name ------------------------------------------------------------------------------

type tool struct{ name, by string }

var toolsContract = extkit.NewKeyedContract[tool]("mcp.tools/v1", func(t tool) string { return t.name })

func toolsExt(name string, isDefault bool, tools ...string) *ext {
	return &ext{name: name, decl: extkit.Contracts{Implements: extkit.IDs("mcp.tools/v1")}, register: func(app core.App) error {
		for _, tn := range tools {
			if isDefault {
				toolsContract.ProvideDefault(app, tool{tn, name})
			} else {
				toolsContract.Provide(app, tool{tn, name})
			}
		}
		return nil
	}}
}

func TestKeyedTools(t *testing.T) {
	app := newApp(t)
	desc := &ext{name: "agentdocs", register: func(app core.App) error {
		toolsContract.WrapKey(app, "example_manifest", func(in tool) tool { in.by += "+docs"; return in })
		return nil
	}}
	lines := setup(t, app, extkit.Choices{"mcp.tools/v1": map[string]string{"needer_send": "mymail"}},
		toolsExt("deploy", true, "example_manifest", "example_upload"), // the contract's tools, as defaults
		toolsExt("fastpush", false, "example_upload"),                  // replaces one tool
		toolsExt("needer", false, "needer_send"),
		toolsExt("mymail", false, "needer_send"), // two non-defaults for one key: providers: picks
		desc,
	)
	got := map[string]string{}
	for _, tl := range toolsContract.All(app) {
		got[tl.name] = tl.by
	}
	want := map[string]string{"example_manifest": "deploy+docs", "example_upload": "fastpush", "needer_send": "mymail"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("tools = %v, want %v\nlines %q", got, want, lines)
	}
	if !contains(lines, "[example_upload] ← fastpush (default deploy stepped aside)") || !contains(lines, "[needer_send] ← mymail (providers:)") {
		t.Fatalf("lines %q", lines)
	}
}
