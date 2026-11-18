package pocker

import (
	"log/slog"
	"net/http"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/multisite"
)

// SectionConfig is the `pocker` section (pocketone.yaml, ext/config): settings for running instances as isolates.
// Listeners, instances and the firewall are the host's sections (server, instances, firewall).
//
//	pocker:
//	  root: /var/lib/instances    # one folder per instance: <root>/<id>
//	  egress_allow: []            # private networks apps may still reach
//	  max_js_execution: 30s       # instance limit defaults (an instance's own limits override them)
type SectionConfig struct {
	Root          string `yaml:"root" doc:"The folder holding one folder per instance: <root>/<id>."`
	HooksPoolSize *int   `yaml:"hooks_pool_size" doc:"Ignored: each instance has one JS VM (ext/jsvm). Kept so existing configs load."`
	Limits        `yaml:",inline"`
	EgressAllow   []string `yaml:"egress_allow" doc:"Private networks (CIDRs) instances may still connect to. Default none: public addresses only."`
}

// Section is the key of the `pocker` section.
var Section = extkit.NewSectionFunc("pocker", func() SectionConfig {
	d := DefaultConfig()
	return SectionConfig{Root: d.PBRoot, HooksPoolSize: d.HooksPoolSize, Limits: d.Limits}
}, "Runtime provider pocker: instances run as isolates in this process (root folder, hooks pool, egress allow list, instance limit defaults).")

// Provider is pocker as a host extension for ext/multisite: it reads the `pocker` section and provides runtime/v1
// (instances as isolates). The fields are the host application's extensions, as on [Server].
type Provider struct {
	// Standard is the per-app extension set every instance gets (ext/standard's Set; see [Server.Standard]). Required.
	Standard extkit.AppSet
	// Host is the host's contract scope (see [Server.Host]).
	Host *extkit.Scope
	// InstanceRoutes serve the host's contracts for its instances, after the gates (see [Server.InstanceRoutes]).
	InstanceRoutes []func(http.Handler) http.Handler
	Extensions     []Extension
	SetupApp       func(inst Instance, app core.App) error
	OnEvent        func(Event)
	OnStart        func(m *Manager)
	// Resolve makes relative paths in the section absolute (e.g. ext/config's Config.ResolvePath, relative to the
	// config file); nil = as written (relative to the working directory).
	Resolve func(string) string
}

var _ multisite.Extension = (*Provider)(nil)

// Name implements multisite.Extension.
func (p *Provider) Name() string { return "pocker" }

// Contracts implements extkit.Declarer: pocker is runtime/v1 (and remote/v1 for its instances, with [Remote]), and
// runs the instances instance.source/v1 supplies.
func (p *Provider) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("runtime/v1"), Requires: extkit.IDs("instance.source/v1")}
}

// RegisterHost implements multisite.Extension: it reads the `pocker` section and makes h run instances as isolates.
func (p *Provider) RegisterHost(h *multisite.Host) error {
	sc, _ := Section.Get()
	cfg := DefaultConfig()
	root := sc.Root
	if p.Resolve != nil {
		root = p.Resolve(root)
	}
	cfg.PBRoot, cfg.Dev, cfg.Limits, cfg.EgressAllow = root, h.Dev, sc.Limits, sc.EgressAllow
	cfg.TLS = h.Listeners.TLS // the server section's certificate files (autotls checks them)
	if sc.HooksPoolSize != nil {
		cfg.HooksPoolSize = sc.HooksPoolSize
	}
	if h.Logger == nil {
		h.Logger = slog.Default()
	}
	s := &Server{Config: cfg, Standard: p.Standard, Host: p.Host, InstanceRoutes: p.InstanceRoutes, Extensions: p.Extensions, SetupApp: p.SetupApp, OnEvent: p.OnEvent, OnStart: p.OnStart, Logger: h.Logger}
	return s.RegisterHost(h)
}
