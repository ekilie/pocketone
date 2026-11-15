package pocker

import (
	"log/slog"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/multisite"
	"github.com/pockethost/pocketone/ext/pluginkit"
)

// SectionConfig is the `pocker` section (pocketone.yaml, ext/config): settings for running instances in-process.
// Listeners, instances and the firewall are the host's sections (server, instances, firewall).
//
//	pocker:
//	  root: /var/lib/instances    # one folder per instance: <root>/<id>
//	  egress_allow: []            # private networks apps may still reach
//	  max_js_execution: 30s       # instance limit defaults (an instance's own limits override them)
type SectionConfig struct {
	Root          string `yaml:"root" doc:"The folder holding one folder per instance: <root>/<id>."`
	HooksPoolSize *int   `yaml:"hooks_pool_size" doc:"JS runtimes kept ready per instance (0: a new one for each hook run)."`
	Limits        `yaml:",inline"`
	EgressAllow   []string `yaml:"egress_allow" doc:"Private networks (CIDRs) instances may still connect to. Default none: public addresses only."`
}

// Section is the key of the `pocker` section.
var Section = pluginkit.NewSectionFunc("pocker", func() SectionConfig {
	d := DefaultConfig()
	return SectionConfig{Root: d.PBRoot, HooksPoolSize: d.HooksPoolSize, Limits: d.Limits}
}, "Runtime provider pocker: instances run in-process (root folder, hooks pool, egress allow list, instance limit defaults).")

// Provider is pocker as a runtime provider for ext/multisite (`provider: pocker`). The fields are the host application's
// extensions, as on [Server].
type Provider struct {
	Plugins  []Plugin
	SetupApp func(inst Instance, app core.App) error
	OnEvent  func(Event)
	OnStart  func(m *Manager)
	// Resolve makes relative paths in the section absolute (e.g. ext/config's Config.ResolvePath, relative to the
	// config file); nil = as written (relative to the working directory).
	Resolve func(string) string
}

var _ multisite.Provider = (*Provider)(nil)

// Name implements multisite.Provider.
func (p *Provider) Name() string { return "pocker" }

// Prepare implements multisite.Provider: it reads the `pocker` section and makes h run its instances in-process.
func (p *Provider) Prepare(h *multisite.Host) error {
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
	s := &Server{Config: cfg, Plugins: p.Plugins, SetupApp: p.SetupApp, OnEvent: p.OnEvent, OnStart: p.OnStart, Logger: h.Logger}
	return s.prepare(h)
}
