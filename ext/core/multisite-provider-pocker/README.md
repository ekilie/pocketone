# Pocker

Many PocketOne instances in one process (like Docker, but for PocketOne), routed by hostname. Each instance is a
normal app folder and starts on its first request, in tens of milliseconds. Pocker is a runtime provider for
[Multisite](../multisite/README.md); its peer [Docker](../multisite-provider-docker/README.md) runs each instance in a container instead.

## Install

Pocker is built into `pocketone`. Name it as the provider in `pocketone.yaml` and `pocketone serve` hosts many
instances instead of one app:

```yaml
provider: pocker
```

In your own Go build, register it as a provider ([`examples/pocketone/multi.go`](../../../examples/pocketone/multi.go)
is the stock binary's version), or run a `pocker.Server` on its own as [the example](#example) does:

```go
multisite.RegisterProvider(&pocker.Provider{Resolve: cfg.ResolvePath, Plugins: []pocker.Plugin{
	pitr.New(pitr.Config{}), // any PocketOne plugin, on every instance
}})
```

The module is `github.com/pockethost/pocketone/ext/core/multisite-provider-pocker`.

## Configure

The `pocker` section sets where instances live, the JS pool, outbound network and the default limits for every
instance (keys below). Listeners, instances and the firewall are the [Multisite](../multisite/README.md) sections (`server`,
`instances`, `firewall`).

```yaml
provider: pocker
instances: { file: ./instances.yaml }
pocker:
  root: ./pb_root           # one folder per instance: <root>/<id>
  hooks_pool_size: 2
  egress_allow: []          # private networks instances may still reach
  max_js_execution: 30s     # default limits; each instance can override them
  max_js_allocation: 64MiB
```

An instance overrides any limit in its own `limits` (missing fields keep the default):

```yaml
instances:
  - id: shop
    hosts: [shop.example.com]
    limits: { max_js_execution: 60s, max_concurrent_js: 32 }
```

Every instance runs the PocketOne code in the binary; an instance's `version` is ignored.

## Use

### Try it with any hostname

```yaml
# pocketone.yaml
server: { http: "127.0.0.1:8090" }
provider: pocker
instances: { dev: true }
pocker: { root: ./pb_root }
```

```sh
pocketone serve
curl -H 'Host: blog.localhost' http://127.0.0.1:8090/api/health   # creates and serves ./pb_root/blog.localhost
```

In dev mode a hostname no instance maps becomes an instance named after it, its folder is made on first use, and it
runs in dev mode. In production only the hostnames in the instances file are served (see
[Multisite](../multisite/README.md#the-instances-file)).

### Each instance

- A normal app folder, `<root>/<id>`: `pb_data`, `pb_hooks`, `pb_migrations`, `pb_public`. Its dashboard is at
  `https://<its hostname>/_/`.
- Its JS hooks see only its own `secrets`, through `$os.getenv` and `process.env`.
- A change to its `secrets`, `encryption_key`, `access`, `limits`, `dev`, `dir`, `superusers`, `private`, `plugins` or `remote`
  restarts its app on the next request, after in-flight requests finish. A hostname change doesn't restart it.
- A change in `pb_hooks` restarts just that instance. A broken hooks file affects only that instance (below).

### Broken hook files

In dev mode, a `pb_hooks` file that fails to load is logged and skipped, and the rest of the instance runs, so you can
fix it live. Otherwise an instance doesn't start with a broken file.

When a running instance's `pb_hooks` change, the new code starts beside the running app and takes over only once it
has started. If it fails to load, the previous code keeps serving and the error goes to the instance log, so a bad
edit never takes a working instance down.

### Logs

The app's own logs (the dashboard's Logs) are per instance, in its `pb_data`. `console.*` from JS hooks goes to
`<instance>/logs/exec.log` as JSON lines (`message`, `stream`, `time`), rotated to `exec.log.1` at 10 MiB. A start
failure is written there too.

## Integrations

### Works with

| Plugin | How they fit |
|---|---|
| [Multisite](../multisite/README.md) | Routing, listeners, instances file and gates. Pocker is one of its runtime providers. |
| [Sandbox](../sandbox/README.md) | Every instance's JS hooks, SQL and outbound network run in it, so one instance can't reach another or the host. |
| [Firewall](../multisite-firewall/README.md) | Peer allow-list and rate limits in front of every instance; per-instance limits in `plugins.firewall`. |
| [Auto TLS](../autotls/README.md) | In the stock binary: certificates for the hostnames instances map (`autotls` section). |
| [Server Timing](../multisite-servertiming/README.md) | In the stock binary: Server-Timing headers on every response. |
| [Mailtrap](../mailtrap/README.md) | In the stock binary: dev instances with SMTP off send mail to a local inbox. |
| [Edge](../multisite-edge/README.md) | Instances another edge runs are forwarded there instead of started here. |
| [Stats](../multisite-stats/README.md) | Measures disk use and last request for each instance under `root`. |
| PocketOne plugins | In your own build, any plugin such as [PITR](../pitr/README.md), [S3 Fallback](../multisite-s3-fallback/README.md) or [Mail Relay](../multisite-mailrelay/README.md) runs on every instance, and an instance's `plugins.<name>` settings override the plugin's defaults. `enabled: false` turns it off for that instance. |

### Building on Pocker

- **Plugins.** `pocker.Provider.Plugins` (or `pocker.Server.Plugins`) takes any `pluginkit.Plugin`, registered on each
  instance's app before it starts with that instance's `plugins.<name>` settings. Host-level code can implement
  `Configurer`, `AppIniter`, `JSIniter`, `Starter`, or a host `Gate`, `Wrapper` or `TLSProvider`. See
  [POCKER.md](POCKER.md#plugins).
- **Setup per instance.** `SetupApp(inst, app)` runs after migrations and before the app serves, e.g. to add a route
  checked against `inst.Private`.
- **Manager.** `OnStart` hands host code the `Manager`: `WithInstanceStopped(ctx, id, fn)` changes an instance's files
  while it's stopped (deploys, restores), `StartInstance`, `Running`, `InstanceDir`, `LogInstance`.
- **Restarts.** `app.Restart()` (a backup restore, PITR) restarts only that instance.

## Example

A Pocker build with one extension: a Go route and an admin dashboard page on every instance, in dev mode. Run it from
this folder with `go run ./example`, then open `http://blog.localhost:8090/_/` (login `admin@example.com` /
`password`).

```go
// Command pocker-extension is a Pocker build with one extension: a Go route and an admin dashboard page,
// added to every instance. See docs/plugins.md.
//
// Logins (ext/core/devusers, local testing only): dashboard admin@example.com / password, app user user@example.com / password.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/core/devusers"
	"github.com/pockethost/pocketone/ext/core/multisite-provider-pocker"
)

//go:embed ui
var uiFiles embed.FS

func main() {
	cfg := pocker.DefaultConfig()
	cfg.Dev = true // any hostname; drop this for production (instances.yaml required)

	ui, _ := fs.Sub(uiFiles, "ui")

	srv := &pocker.Server{
		Config: cfg,
		// Runs for every instance's app after its migrations, before it serves.
		SetupApp: func(inst pocker.Instance, app core.App) error {
			// local testing: every instance gets superuser admin@example.com and user user@example.com, password "password"
			if err := devusers.Ensure(app); err != nil {
				return err
			}
			app.OnServe().BindFunc(func(se *core.ServeEvent) error {
				// a Go route, superuser only
				se.Router.GET("/api/hello", func(e *core.RequestEvent) error {
					return e.JSON(http.StatusOK, map[string]string{"instance": inst.ID})
				}).Bind(apis.RequireSuperuserAuth())

				// an admin dashboard extension: ui/main.js is loaded by the dashboard before its router starts
				se.UIExtensions = append(se.UIExtensions, core.UIExtension{Name: "hello", FS: ui})
				return se.Next()
			})
			return nil
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := srv.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
```

## Untrusted instances

Pocker runs every instance's JS hooks in the [sandbox](../sandbox/README.md). For why that's safe enough to host code
from people you don't know, and where it isn't, see [How is this safe?](../sandbox/README.md#how-is-this-safe).
Memory is the main limit: only one-shot allocations are capped, so a hook that grows memory slowly affects the whole
process. [POCKER.md](POCKER.md#sandbox) lists what the sandbox covers.

## Reference

[POCKER.md](POCKER.md) is the full guide: production `pocketone.yaml`, every instance field, plugin interfaces, the
sandbox, PocketBase changes and versions.

## License

MIT
