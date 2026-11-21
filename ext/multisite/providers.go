package multisite

import (
	"errors"
	"log/slog"
	"net/http"

	"pocketone.dev/ext/config"
	"pocketone.dev/ext/extkit"
)

// Config is the `multisite` section (pocketone.yaml): having it makes the binary a multi-instance host instead of a
// single app. Which runtimes run the instances is `providers: { runtime/v1: [pocker, docker] }` (default: every
// linked runtime that's configured, in link order).
//
//	multisite: {}
type Config struct{}

// Section is the key of the `multisite` section.
var Section = extkit.NewSection("multisite", Config{}, "Serve many instances (a multisite host) instead of a single app. The runtimes are the providers of runtime/v1.")

// InstancesConfig is the `instances` section: where a host's instances come from when the binary doesn't supply a
// source of its own (e.g. a control plane's mirror).
//
//	instances:
//	  file: instances.yaml   # instances: [{id, hosts, version, …}]
//	  dev: false             # also serve unknown hostnames (folder created on first use)
type InstancesConfig struct {
	File string `yaml:"file" doc:"The instances file (id, hosts and settings for each instance), reloaded when it changes."`
	Dev  bool   `yaml:"dev" doc:"Also serve unknown hostnames: each becomes an instance named after it, in dev mode, with its folder made on first use."`
}

// Instances is the section key of InstancesConfig.
var Instances = extkit.NewSection("instances", InstancesConfig{File: "instances.yaml"}, "Where a multi-instance host's instances come from (a YAML file), and dev mode (serve unknown hostnames).")

// Options are what the binary adds to a host built from config.
type Options struct {
	Source   InstanceSource                                              // nil = the `instances` section's file
	Forward  func(w http.ResponseWriter, r *http.Request, inst Instance) // the fleet hook (ext/edge)
	Gates    []Gate                                                      // after the gate/v1 providers'
	Wrappers []func(http.Handler) http.Handler                           // inside the http.wrap/v1 providers'
	Logger   *slog.Logger
	// Resolve makes a relative path from the config file relative to the file's folder (ext/config's
	// Config.ResolvePath); nil = paths as written.
	Resolve func(string) string
	// Extensions are host extensions (see [Extension]): runtimes (pocker, docker), servertiming, autotls, ...
	// registered after the linked ones ([Link]), in order.
	Extensions []Extension
	// Choices is the `providers:` section (the host-scope contracts in it apply here).
	Choices extkit.Choices
}

// ErrNoProvider means the config has no `multisite` section: the binary should serve a single app instead.
var ErrNoProvider = errors.New("no multisite section: not a multi-instance host")

// FromConfig builds a host from the loaded config sections: listeners from `server`, instances from `instances`
// (unless o.Source), then every host extension (linked, then o.Extensions) and the seal. It returns ErrNoProvider
// without a `multisite` section, and an error when no runtime/v1 provider is configured.
func FromConfig(o Options) (*Host, error) {
	if _, on := Section.Get(); !on {
		return nil, ErrNoProvider
	}
	if app, found := config.App.Get(); found && (app.Dir != "" || app.HooksDir != "" || app.MigrationsDir != "") {
		return nil, errors.New("app: applies to a single app; on a multisite host each instance has its own folder under its runtime's root")
	}
	if srv, _ := config.Server.Get(); srv.PublicDir != "" || srv.IndexFallback != nil || srv.PagesDir != "" || srv.HooksWatch != nil || srv.Automigrate != nil {
		return nil, errors.New("server: public_dir, index_fallback, pages_dir, hooks_watch and automigrate apply to a single app; on a multisite host each instance has its own")
	}
	resolve := o.Resolve
	if resolve == nil {
		resolve = func(s string) string { return s }
	}
	log := o.Logger
	if log == nil {
		log = slog.Default()
	}
	srv, _ := config.Server.Get()
	inst, _ := Instances.Get()
	h := &Host{Logger: log, Dev: inst.Dev, Forward: o.Forward, Wrappers: o.Wrappers, Gates: o.Gates}
	h.Listeners = Listeners{
		Listen:          srv.HTTPS,
		PlainListen:     srv.Plain,
		TLS:             TLSConfig{Cert: resolve(srv.TLS.Cert), Key: resolve(srv.TLS.Key)},
		ShutdownTimeout: srv.ShutdownTimeout,
	}
	switch {
	case srv.HTTPS != "" && srv.HTTP != "":
		h.Listeners.TLS.HTTPListen = srv.HTTP // plain HTTP redirects to HTTPS
	case srv.HTTPS == "":
		h.Listeners.Listen = srv.HTTP // plain HTTP only
	}
	if h.Listeners.Listen == "" {
		h.Listeners.Listen = "127.0.0.1:8090"
	}
	h.Source = o.Source
	if h.Source == nil {
		h.Source = &YAMLSource{Path: resolve(inst.File), Required: !inst.Dev}
	}
	h.Directory = NewDirectory(log)
	if err := h.Use(append(Linked(), o.Extensions...)...); err != nil {
		return nil, err
	}
	if _, err := h.Setup(o.Choices); err != nil {
		return nil, err
	}
	if h.Runtime == nil {
		return nil, errors.New("multisite: no runtime: link one (pocker, docker) and configure it, or check `providers: { runtime/v1: ... }`")
	}
	return h, nil
}
