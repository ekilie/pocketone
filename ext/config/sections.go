package config

import (
	"time"

	"github.com/pockethost/pocketone/ext/extkit"
)

// ServerConfig is the `server` section: how the binary listens.
//
// Single-app mode: http, https and origins are the serve command's --http, --https and --origins defaults (flags
// still win). Multi-instance mode (a provider): https is the main address (TLS from tls or a TLS extension), http then
// redirects plain HTTP to HTTPS (http alone serves plain HTTP), plain lists extra plain-HTTP addresses with no
// redirect (e.g. a private network other edges forward on), and shutdown_timeout bounds in-flight requests at stop.
//
//	server:
//	  https: ":443"
//	  http: ":80"
//	  plain: [":8080"]
//	  tls: { cert: /etc/certs/cert.pem, key: /etc/certs/key.pem }
type ServerConfig struct {
	HTTP            string        `yaml:"http" doc:"Plain HTTP address, such as 127.0.0.1:8090. Single app: the --http default (the flag wins). Many instances: redirects to https when https is set."`
	HTTPS           string        `yaml:"https" doc:"HTTPS address. Single app: the --https default (the flag wins). Many instances: the main address."`
	Origins         []string      `yaml:"origins" doc:"Allowed CORS origins. Single app: the --origins default (the flag wins)."`
	Plain           []string      `yaml:"plain" doc:"Many instances only: extra plain HTTP addresses with no redirect."`
	TLS             ServerTLS     `yaml:"tls"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" doc:"Many instances only: how long in-flight requests get to finish at stop."`
}

// ServerTLS is a certificate pair, reloaded when the files change.
type ServerTLS struct {
	Cert string `yaml:"cert" doc:"Many instances only: the certificate file, reloaded when it changes."`
	Key  string `yaml:"key" doc:"Many instances only: the private key file, reloaded when it changes."`
}

// AppConfig is the `app` section: the one app of single-app mode (the root flags as config; flags still win). It
// doesn't apply when a runtime provider runs many instances (each has its own folder under the provider's root).
//
//	app:
//	  dir: ./pb_data
//	  dev: false
//	  encryption_env: PB_ENCRYPTION_KEY
//	  hooks_dir: ./pb_hooks
//	  migrations_dir: ./pb_migrations
//	  public_dir: ./pb_public
//	  env_allow: [STRIPE_KEY]       # .env names the app (and its JS hooks) may read; default none
type AppConfig struct {
	Dir           string        `yaml:"dir" doc:"The data folder (pb_data). Default: pb_data next to the executable. The --dir flag wins."`
	Dev           *bool         `yaml:"dev" doc:"Dev mode: logs and SQL statements printed to the console. The --dev flag wins."`
	EncryptionEnv string        `yaml:"encryption_env" doc:"Name of the environment variable holding a 32-character key that encrypts the app settings. The --encryptionEnv flag wins."`
	QueryTimeout  time.Duration `yaml:"query_timeout" doc:"Default timeout for SELECT queries, such as 30s. The --queryTimeout flag wins."`
	HooksDir      string        `yaml:"hooks_dir" doc:"The JS hooks folder. Default: pb_hooks next to pb_data. The --hooksDir flag wins."`
	HooksWatch    *bool         `yaml:"hooks_watch" doc:"Restart the app when a pb_hooks file changes. On unless false. The --hooksWatch flag wins."`
	HooksPool     int           `yaml:"hooks_pool" doc:"How many JS runtimes are kept ready for hooks. Default 15. The --hooksPool flag wins."`
	MigrationsDir string        `yaml:"migrations_dir" doc:"The migrations folder. Default: pb_migrations next to pb_data. The --migrationsDir flag wins."`
	Automigrate   *bool         `yaml:"automigrate" doc:"Write a migration file when you change collections in the dashboard. On unless false. The --automigrate flag wins."`
	PublicDir     string        `yaml:"public_dir" doc:"The static files folder. Default: pb_public next to the executable. The --publicDir flag wins."`
	IndexFallback *bool         `yaml:"index_fallback" doc:"Serve index.html for a missing static path, for single-page apps. On unless false. The --indexFallback flag wins."`
	EnvAllow      []string      `yaml:"env_allow" doc:"Names from the .env the app and its JS hooks may read. Default none."`
}

// Server and App are the sections this package owns. (A multisite host is the `multisite` section, ext/multisite;
// its runtimes are `providers: { runtime/v1: [...] }`.)
var (
	Server   = extkit.NewSectionFunc("server", func() ServerConfig { return ServerConfig{} }, "How the binary listens: http, https and CORS origins (single-app mode: the serve command's defaults).")
	App      = extkit.NewSectionFunc("app", func() AppConfig { return AppConfig{} }, "The single app: data dir, dev mode, encryption, hooks, migrations, public dir, and the .env names it may read (env_allow). Not used on a multisite host.")
)
