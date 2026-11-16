package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pockethost/pocketone/ext/autotls"
	"github.com/pockethost/pocketone/ext/config"
	"github.com/pockethost/pocketone/ext/docker"
	"github.com/pockethost/pocketone/ext/edge"
	_ "github.com/pockethost/pocketone/ext/firewall" // turns on with a firewall / rate_limits section
	"github.com/pockethost/pocketone/ext/mailtrap"
	"github.com/pockethost/pocketone/ext/mothership/mirror"
	"github.com/pockethost/pocketone/ext/multisite"
	"github.com/pockethost/pocketone/ext/pocker"
	"github.com/pockethost/pocketone/ext/servertiming"
	"github.com/pockethost/pocketone/ext/standard"
	"github.com/pockethost/pocketone/ext/stats"
)

// serveMany runs the binary as a multi-instance host when pocketone.yaml names a provider: instances from the
// `instances` section, or mirrored from a community mothership (`mothership` section), routed by hostname, run by
// pocker (in-process) or docker (containers); other edges from the `edges` section and the mothership. It reports
// false when no provider is configured, so main serves a single app as stock PocketBase does.
func serveMany(cfg *config.Config) (handled bool, err error) {
	// every instance gets the standard set (tokens, mcp, pitr, ...), each switchable per instance in extensions.<name>
	set := standard.New()
	set.Providers = standard.ConfiguredProviders()
	log := slog.Default()
	// host extensions, in order: wrappers first (servertiming outermost), then the runtimes in preference order
	// (`providers: { runtime/v1: [...] }` picks and orders them; docker only runs with a `docker` section)
	opts := multisite.Options{Resolve: cfg.ResolvePath, Logger: log, Choices: set.Providers, Extensions: []multisite.Extension{
		servertiming.New(), // http.wrap/v1: outermost, so its header includes everything below
		autotls.New(),      // tls/v1 from the `autotls` section, whatever the runtime
		&pocker.Provider{Resolve: cfg.ResolvePath, Standard: set, Extensions: []pocker.Extension{
			mailtrap.New(), // dev instances with SMTP off: mail goes to the inbox at http://127.0.0.1:8025
		}},
		&docker.Provider{Resolve: cfg.ResolvePath},
	}}
	edges, err := edge.FromSection(log)
	if err != nil {
		return true, err
	}
	m, err := mirror.FromSection(log)
	if err != nil {
		return true, err
	}
	var background []func(context.Context)
	if m != nil {
		opts.Extensions = append(opts.Extensions, m) // instance.source/v1: the instances come from the mothership
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
	if len(h.Runtimes) == 1 && h.Runtimes[0].Name() == "pocker" {
		// pocker alone serves every instance in this process: it names itself (docker names each container's release)
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
