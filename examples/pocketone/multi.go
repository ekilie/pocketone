package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"pocketone.dev/ext/autotls"
	"pocketone.dev/ext/config"
	"pocketone.dev/ext/docker"
	"pocketone.dev/ext/edge"
	_ "pocketone.dev/ext/firewall" // turns on with a firewall / rate_limits section
	"pocketone.dev/ext/mailtrap"
	cachev1 "pocketone.dev/ext/contracts/cache/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/mothership/client"
	"pocketone.dev/ext/multisite"
	"pocketone.dev/ext/pocker"
	"pocketone.dev/ext/servertiming"
	"pocketone.dev/ext/standard"
	"pocketone.dev/ext/stats"
)

// serveMany runs the binary as a multi-instance host when pocketone.yaml names a provider: instances from the
// `instances` section, or looked up from a community mothership (`mothership` section), routed by hostname, run by
// pocker (in-process) or docker (containers); other edges from the `edges` section and the mothership. It reports
// false when no provider is configured, so main serves a single app as stock PocketBase does.
func serveMany(cfg *config.Config) (handled bool, err error) {
	// every instance gets the standard set (tokens, mcp, pitr, ...), each switchable per instance in extensions.<name>
	set := standard.New()
	set.Providers = standard.ConfiguredProviders()
	log := slog.Default()
	// one runtime per host (runtime/v1): docker with a `docker` section, else pocker; `providers: { runtime/v1:
	// <name> }` overrides. Only that one is registered, so the other sets nothing up.
	runtimeName := "pocker"
	if _, on := docker.Section.Get(); on {
		runtimeName = "docker"
	}
	if name, ok := set.Providers["runtime/v1"].(string); ok {
		runtimeName = name
	}
	var runtime multisite.Extension
	switch runtimeName {
	case "pocker":
		runtime = &pocker.Provider{Resolve: cfg.ResolvePath, Standard: set, Extensions: []pocker.Extension{
			mailtrap.New(), // dev instances with SMTP off: mail goes to the inbox at http://127.0.0.1:8025
		}}
	case "docker":
		runtime = &docker.Provider{Resolve: cfg.ResolvePath}
	default:
		return true, fmt.Errorf("providers: runtime/v1: %q isn't a runtime (pocker, docker)", runtimeName)
	}
	// host extensions, in order: wrappers first (servertiming outermost), then the runtime
	opts := multisite.Options{Resolve: cfg.ResolvePath, Logger: log, Choices: set.Providers, Extensions: []multisite.Extension{
		servertiming.New(), // http.wrap/v1: outermost, so its header includes everything below
		autotls.New(),      // tls/v1 from the `autotls` section, whatever the runtime
		runtime,
	}}
	edges, err := edge.FromSection(log)
	if err != nil {
		return true, err
	}
	m, err := client.FromSection(log)
	if err != nil {
		return true, err
	}
	var background []func(context.Context)
	if m != nil {
		// the host's own cache/v1 (the store every instance shares): the client keeps its answers there
		opts.Extensions = append(opts.Extensions, multisite.Provides("cache", func(s *extkit.Scope) {
			cachev1.Contract.ProvideIn(s, set.Cache.Host(s))
		}))
		// instance.source/v1 and instance.lookup/v1: each hostname is asked of the mothership the first time it's
		// requested, then followed on one stream
		opts.Extensions = append(opts.Extensions, m)
		if edges != nil {
			edges = edge.MultiProvider{edges, m}
		} else {
			edges = m
		}
	}
	if edges != nil {
		opts.Forward = edge.NewRouter(edge.RouterConfig{Edges: edges, Logger: log}).Forward
		if r, ok := edges.(edge.Runner); ok {
			background = append(background, r.Run)
		}
	}
	h, err := multisite.FromConfig(opts)
	if errors.Is(err, multisite.ErrNoProvider) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	h.Background = append(h.Background, background...)
	if h.Runtime.Name() == "pocker" {
		// pocker serves every instance in this process: it names itself (docker names each container's release)
		poweredBy := "Pocker " + pocker.Version()
		h.Wrappers = append(h.Wrappers, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Powered-By", poweredBy)
				next.ServeHTTP(w, r)
			})
		})
	}

	// instance data: disk used and last request, measured and logged on the `lifecycle` section's schedule
	if root := h.DataRoot; root != "" {
		lc := &stats.Lifecycle{Directory: h.Directory, Folder: func(i multisite.Instance) string {
			if i.Dir != "" {
				return cfg.ResolvePath(i.Dir)
			}
			return filepath.Join(root, i.ID)
		}, Logger: log}
		h.Gates = append(h.Gates, lc)
		h.Background = append(h.Background, lc.Run)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return true, h.Run(ctx)
}
