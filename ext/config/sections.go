package config

import (
	"time"

	"github.com/pockethost/pocketone/ext/extkit"
)

// ServerConfig is the `server` section: the serve command's settings, such as how the binary listens.
//
// Single-app mode: each key is the default of the serve flag of the same name (http is --http, public_dir is
// --publicDir, ...; flags still win), and public_dir, index_fallback, pages_dir, hooks_watch and automigrate apply to
// the single app only. Multi-instance mode (a provider): https is the main address (TLS from tls or a TLS extension),
// http then redirects plain HTTP to HTTPS (http alone serves plain HTTP), plain lists extra plain-HTTP addresses with no
// redirect (e.g. a private network other edges forward on), and shutdown_timeout bounds in-flight requests at stop.
//
//	server:              # a single app
//	  http: 127.0.0.1:8090
//	  public_dir: ./pb_public
//
//	server:              # many instances
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
	PublicDir       string        `yaml:"public_dir" doc:"Single app only: the static files folder. Default: pb_public next to the executable. The --publicDir flag wins."`
	IndexFallback   *bool         `yaml:"index_fallback" doc:"Single app only: serve index.html for a missing static path, for single-page apps. On unless false. The --indexFallback flag wins."`
	PagesDir        string        `yaml:"pages_dir" doc:"Single app only: the PocketPages folder, served when it exists. Default: pb_pages next to pb_data. The --pagesDir flag wins."`
	HooksWatch      *bool         `yaml:"hooks_watch" doc:"Single app only: restart the app when a pb_hooks file changes. On unless false. The --hooksWatch flag wins."`
	Automigrate     *bool         `yaml:"automigrate" doc:"Single app only: write a migration file when you change collections in the dashboard. On unless false. The --automigrate flag wins."`
}

// ServerTLS is a certificate pair, reloaded when the files change.
type ServerTLS struct {
	Cert string `yaml:"cert" doc:"Many instances only: the certificate file, reloaded when it changes."`
	Key  string `yaml:"key" doc:"Many instances only: the private key file, reloaded when it changes."`
}

// AppConfig is the `app` section: the one app of single-app mode, as every command that opens it reads it (serve,
// migrate, superuser, pitr, token, oauth, template); each key is the default of those commands' flag of the same
// name, and flags still win. Settings only serve reads are in `server`. It doesn't apply when a runtime provider runs
// many instances (each has its own folder under the provider's root).
//
//	app:
//	  dir: ./pb_data
//	  dev: false
//	  encryption_env: PB_ENCRYPTION_KEY
//	  jsvm: quickjs                 # or goja: PocketBase's own JS engine, for pure PocketBase compatibility
//	  hooks_dir: ./pb_hooks
//	  migrations_dir: ./pb_migrations
//	  env_allow: [STRIPE_KEY]       # .env names the app (and its JS hooks) may read; default none
//	  js_limits:                    # QuickJS limits; each defaults when left out
//	    memory: 128MiB
//	    task_time: 10s
//	    task_wall: 60s
type AppConfig struct {
	Dir           string        `yaml:"dir" doc:"The data folder (pb_data). Default: pb_data next to the executable. The --dir flag wins."`
	Dev           *bool         `yaml:"dev" doc:"Dev mode: logs and SQL statements printed to the console. The --dev flag wins."`
	EncryptionEnv string        `yaml:"encryption_env" doc:"Name of the environment variable holding a 32-character key that encrypts the app settings. The --encryptionEnv flag wins."`
	QueryTimeout  time.Duration `yaml:"query_timeout" doc:"Default timeout for SELECT queries, such as 30s. The --queryTimeout flag wins."`
	HooksDir      string        `yaml:"hooks_dir" doc:"The JS hooks folder. Default: pb_hooks next to pb_data. The --hooksDir flag wins."`
	JSVM          string        `yaml:"jsvm" doc:"The JS engine for pb_hooks and pb_migrations: quickjs (PocketOne's, the default) or goja (PocketBase's own, for pure PocketBase compatibility: pages, $mcp, $apiTokens and JS contracts are off). The --jsvm flag wins."`
	HooksPool     int           `yaml:"hooks_pool" doc:"Only with jsvm: goja: how many JS runtimes are kept ready for hooks. Default 15. The --hooksPool flag wins."`
	MigrationsDir string        `yaml:"migrations_dir" doc:"The migrations folder. Default: pb_migrations next to pb_data. The --migrationsDir flag wins."`
	EnvAllow      []string      `yaml:"env_allow" doc:"Names from the .env the app and its JS hooks may read. Default none."`
	JSLimits      JSLimits      `yaml:"js_limits" doc:"Limits on the JS engine (jsvm: quickjs): a runaway hook stops with an error and the app keeps serving."`
}

// JSLimits are the QuickJS engine's limits for the app (ext/jsvm's VMConfig); 0 keeps each default.
type JSLimits struct {
	Memory   Size          `yaml:"memory" doc:"The app's JS memory: the JS heap plus the Go objects JS holds, such as 128MiB. Default 64MiB."`
	TaskTime time.Duration `yaml:"task_time" doc:"JS time one task (a request, a hook, a cron job) may use, Go calls and waits not counted, such as 10s. Default 5s."`
	TaskWall time.Duration `yaml:"task_wall" doc:"Wall time one task may take, Go calls included, such as 60s. Default 30s."`
}

// Server and App are the sections this package owns. (A multisite host is the `multisite` section, ext/multisite;
// its runtimes are `providers: { runtime/v1: [...] }`.)
var (
	Server = extkit.NewSectionFunc("server", func() ServerConfig { return ServerConfig{} }, "The serve command: http, https and CORS origins, and for a single app its static files, pages, hooks watching and automigrate.")
	App    = extkit.NewSectionFunc("app", func() AppConfig { return AppConfig{} }, "The single app, for every command that opens it: data dir, dev mode, encryption, JS engine, hooks, migrations, and the .env names it may read (env_allow). Not used on a multisite host.")
)
