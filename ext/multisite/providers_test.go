package multisite

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pocketone.dev/ext/config"
	gatev1 "pocketone.dev/ext/contracts/gate/v1"
	runtimev1 "pocketone.dev/ext/contracts/runtime/v1"
	"pocketone.dev/ext/extkit"
)

// fakeRuntime is a runtime/v1 provider: a host extension that provides echoRT named name.
type fakeRuntime struct {
	name string
	off  bool
}

func (p *fakeRuntime) Name() string { return p.name }
func (p *fakeRuntime) RegisterHost(h *Host) error {
	if !p.off {
		runtimev1.Contract.ProvideIn(h.Scope, echoRT{p.name})
	}
	return nil
}

// testGate is a gate/v1 provider that turns itself on from a flag (as ext/firewall does from its section).
type testGate struct{ on *bool }

func (g testGate) Name() string { return "testgate" }
func (g testGate) RegisterHost(h *Host) error {
	if *g.on {
		gatev1.Contract.ProvideIn(h.Scope, allowAll{})
	}
	return nil
}

type echoRT struct{ name string }

func (r echoRT) Name() string                                                   { return r.name }
func (echoRT) Serves(Instance) bool                                             { return true }
func (echoRT) Ready() bool                                                      { return true }
func (echoRT) ServeInstance(w http.ResponseWriter, _ *http.Request, _ Instance) {}

type allowAll struct{}

func (allowAll) AllowPeer(*http.Request) bool { return true }
func (allowAll) Admit(http.ResponseWriter, *http.Request, Instance) (func(), bool) {
	return func() {}, true
}

func loadConfig(t *testing.T, yaml string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "pocketone.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(config.Options{ConfigPath: p})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { extkit.SetSectionSource(nil) })
	_ = c
}

func TestFromConfig(t *testing.T) {
	gateOn := false
	Link(testGate{&gateOn})
	fake := []Extension{&fakeRuntime{name: "fake"}}

	// no multisite section: the binary serves a single app
	loadConfig(t, "server: { http: 127.0.0.1:8090 }\n")
	if _, err := FromConfig(Options{Extensions: fake}); !errors.Is(err, ErrNoProvider) {
		t.Fatalf("no multisite section: %v", err)
	}

	// a host with no runtime configured
	loadConfig(t, "multisite: {}\n")
	if _, err := FromConfig(Options{Extensions: []Extension{&fakeRuntime{name: "fake", off: true}}}); err == nil || !strings.Contains(err.Error(), "no runtime") {
		t.Fatalf("no runtime: %v", err)
	}

	// app: is single-app only
	loadConfig(t, "multisite: {}\napp: { dir: ./data }\n")
	if _, err := FromConfig(Options{Extensions: fake}); err == nil || !strings.Contains(err.Error(), "single app") {
		t.Fatalf("app with multisite: %v", err)
	}

	// listeners: https main, http redirects, plain extra; instances from the section; gates
	gateOn = true
	loadConfig(t, `
multisite: {}
server:
  https: ":443"
  http: ":80"
  plain: [":8080"]
  tls: { cert: c.pem, key: k.pem }
instances: { file: inst.yaml, dev: true }
`)
	h, err := FromConfig(Options{Resolve: func(s string) string { return "/etc/x/" + s }, Extensions: fake})
	if err != nil {
		t.Fatal(err)
	}
	l := h.Listeners
	if l.Listen != ":443" || l.TLS.HTTPListen != ":80" || len(l.PlainListen) != 1 || l.TLS.Cert != "/etc/x/c.pem" || l.TLS.Key != "/etc/x/k.pem" {
		t.Fatalf("listeners: %+v", l)
	}
	if src, ok := h.Source.(*YAMLSource); !ok || src.Path != "/etc/x/inst.yaml" || src.Required || !h.Dev {
		t.Fatalf("source: %+v dev %v", h.Source, h.Dev)
	}
	if len(h.Gates) != 1 || h.Runtime == nil || h.Runtime.Name() != "fake" || h.Directory == nil {
		t.Fatalf("gates %d runtime %v", len(h.Gates), h.Runtime)
	}

	// http alone: plain HTTP, no redirect; a source from the binary wins over the section
	gateOn = false
	loadConfig(t, "multisite: {}\nserver: { http: \":9000\" }\n")
	src := &YAMLSource{Path: "mine.yaml"}
	h, err = FromConfig(Options{Source: src, Extensions: fake})
	if err != nil {
		t.Fatal(err)
	}
	if h.Listeners.Listen != ":9000" || h.Listeners.TLS.HTTPListen != "" || h.Source != src || len(h.Gates) != 0 {
		t.Fatalf("http only: %+v source %v gates %d", h.Listeners, h.Source, len(h.Gates))
	}
}

// runtime/v1 is One: a host runs one runtime; two registered conflict unless `providers:` picks.
func TestRuntimeFromProviders(t *testing.T) {
	loadConfig(t, "multisite: {}\n")
	h, err := FromConfig(Options{Extensions: []Extension{&fakeRuntime{name: "docker"}}})
	if err != nil || h.Runtime == nil || h.Runtime.Name() != "docker" {
		t.Fatalf("one: %v %v", h, err)
	}
	two := []Extension{&fakeRuntime{name: "pocker"}, &fakeRuntime{name: "docker"}}
	if _, err := FromConfig(Options{Extensions: two}); err == nil || !strings.Contains(err.Error(), "conflict: runtime/v1") {
		t.Fatalf("two without a pick: %v", err)
	}
	h, err = FromConfig(Options{Extensions: two, Choices: extkit.Choices{"runtime/v1": "docker"}})
	if err != nil || h.Runtime.Name() != "docker" {
		t.Fatalf("picked: %v %v", h, err)
	}
}
