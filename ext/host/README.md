# Host

Many PocketOne instances behind one listener, routed by hostname. Each request's hostname picks an instance, the
request passes the gates (such as the [firewall](../firewall/README.md)), and one runtime provider serves it:
[pocker](../pocker/README.md) runs instances in-process, [docker](../docker/README.md) runs each in its own container.

## Install

Host is built into `pocketone`. It takes over `serve` when `pocketone.yaml` names a `provider`; without one,
`pocketone serve` runs a single app as usual.

```yaml
provider: pocker   # or docker
```

In your own Go build, register the providers you link, build the host from the config and run it.
[`examples/pocketone/multi.go`](../../examples/pocketone/multi.go) is the stock binary's version:

```go
host.RegisterProvider(&pocker.Provider{Resolve: cfg.ResolvePath})
h, err := host.FromConfig(host.Options{Resolve: cfg.ResolvePath})
if errors.Is(err, host.ErrNoProvider) {
	// no provider in pocketone.yaml: serve a single app instead
}
err = h.Run(ctx)
```

The module is `github.com/pockethost/pocketone/ext/host`.

## Configure

A host reads these `pocketone.yaml` sections:

```yaml
provider: pocker                     # which runtime provider runs the instances
server:
  https: ":443"                      # the main address
  http: ":80"                        # redirects to https (alone: serves plain HTTP)
  plain: [":8080"]                   # extra plain-HTTP addresses, no redirect (e.g. a private network)
  tls: { cert: cert.pem, key: key.pem }   # reloaded when the files change; or the autotls section
instances:
  file: ./instances.yaml             # where the instances come from
  dev: false                         # also serve unknown hostnames
pocker: { root: ./pb_root }          # the provider's own section (pocker or docker)
firewall: { allow_cidrs: [10.0.0.0/8] }  # any gate's section turns that gate on
```

- `provider` and `server` belong to [Config](../config/README.md). With no `https` or `http`, the host listens on
  `127.0.0.1:8090`.
- The `instances` section is this plugin's (keys below). In production the instances file is required; with
  `dev: true` it's optional.
- The `app` section is for a single app and is refused when a provider is set: each instance has its own folder under
  the provider's root.

## Use

### The instances file

```yaml
instances:
  - id: blog                         # also the folder name: <root>/blog
    hosts: [blog.example.com, www.blog.example.com]
  - id: shop
    hosts: [shop.example.com]
    secrets: { STRIPE_KEY: sk_live_... }               # the app's environment, seen by its JS hooks
    encryption_key: 0123456789abcdef0123456789abcdef   # encrypts the app's stored settings
    access: { cors_origins: [https://shop.example.com] }
    limits: { max_js_execution: 60s }                  # over the provider's defaults
    plugins: { firewall: { trusted_ips: [203.0.113.0/24] } }   # settings for plugins, by name
  - id: old
    hosts: [old.example.com]
    blocked: { status: 402, message: "Payment required" }
```

The file is checked for changes every 2 seconds; edits apply without restarting the host. Ids are lowercase
letters, digits, dots and dashes. Other fields: `version` and `runtime` (which release or provider runs it), `dir`
(another folder), `dev`, `superusers` (pocker upserts them when the app starts), `private` (for host Go code, never seen by JS) and `remote`
(another server runs it).

### What a request gets

- A hostname no instance maps gets 404, and no folder is created. In dev mode it becomes a new instance named after
  the hostname, in dev mode.
- A `blocked` instance answers every request with its status and message.
- An instance with `remote` set is forwarded there after the gates (a plain proxy to the URL, or
  [Edge](../edge/README.md) for a fleet).
- `GET /_pocker/health` on any hostname reports the host's status, how many instances it knows and how many are
  running.

### Switching provider

Switching between pocker and docker is a config change: both keep instances in `<root>/<id>`. Stop the containers
first when leaving docker (they keep running across a host restart on purpose). Data only moves forward: an instance
started on a newer version than wrote its data is upgraded.

## Integrations

### Works with

| Plugin | How they fit |
|---|---|
| [Pocker](../pocker/README.md) | Runtime provider: every instance runs in this process (`provider: pocker`). |
| [Docker](../docker/README.md) | Runtime provider: each instance runs in its own container (`provider: docker`). |
| [Firewall](../firewall/README.md) | A gate: peer allow-list, client IPs and rate limits. On when the config has `firewall` or `rate_limits`. |
| [Edge](../edge/README.md) | Forwards requests for instances another edge runs (the `edges` section). |
| [Mothership](../mothership/README.md) | Its mirror replaces the instances file as the instance source, and lists the other edges. |
| [Auto TLS](../autotls/README.md) | Certificates for the main listener, issued for the hostnames instances map. |
| [Lifecycle](../lifecycle/README.md) | Measures disk use and last request for each instance. |
| [Server Timing](../servertiming/README.md) | Server-Timing headers for each step of a request. |

### Building on Host

- **Runtime provider.** Implement `host.Provider` (`Name`, `Prepare`) and call `host.RegisterProvider`; `provider:`
  picks it by name. `Prepare` sets `h.Runtime`, a `host.Runtime` (`Name`, `Serves`, `ServeInstance`, `Ready`), with
  optional `Prechecker`, `Syncer`, `Statser` and `Shutdowner`.
- **Instance source.** Implement `host.InstanceSource` (`Snapshot`, `Watch`) to feed instances from anywhere, such as
  a control plane, and pass it as `host.Options.Source`. `YAMLSource` is the built-in one.
- **Gates.** A `host.Gate` checks the peer (`AllowPeer`) and admits each request (`Admit`). `RegisterSectionGate`
  turns one on from its own config sections, as the firewall does; `ClientIPResolver` passes the real client IP on to
  the app.
- **Forwarding.** `host.Options.Forward` serves requests for instances with `remote` set (Edge fills it).
- **Listeners and wrappers.** `Host.TLS` takes a `TLSProvider`, `Host.Wrappers` wrap the whole handler, and
  `Host.Background` runs jobs until shutdown.
- **Per-instance settings.** A plugin reads its own `plugins.<name>` with `inst.PluginConfig(name)` and
  `host.DecodePluginConfig`.

## Example

A toy host: instances from a YAML file, served by a runtime that answers with the instance it resolved. Run it from
this folder with `go run ./example --instances instances.yaml`, then `curl -H 'Host: blog.localhost' localhost:8090/`.

```go
// A minimal host: instances from a YAML file, routed by hostname, served by a toy runtime that answers with the
// instance it resolved. Real hosts plug in a provider instead (ext/pocker in-process, ext/docker in containers).
//
//	go run ./example --instances instances.yaml
//	curl -H 'Host: blog.localhost' localhost:8090/
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pockethost/pocketone/ext/host"
)

type echoRuntime struct{}

func (echoRuntime) Name() string                   { return "echo" }
func (echoRuntime) Serves(inst host.Instance) bool { return true }
func (echoRuntime) Ready() bool                    { return true }
func (echoRuntime) ServeInstance(w http.ResponseWriter, r *http.Request, inst host.Instance) {
	fmt.Fprintf(w, "instance %s (%s%s)\n", inst.ID, r.Host, r.URL.Path)
}

func main() {
	instances := flag.String("instances", "instances.yaml", "instances file: instances: [{id: blog, hosts: [blog.localhost]}]")
	listen := flag.String("listen", "127.0.0.1:8090", "listen address")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	h := &host.Host{Source: &host.YAMLSource{Path: *instances}, Runtime: echoRuntime{}, Listeners: host.Listeners{Listen: *listen}}
	if err := h.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

## How it works

| Piece | What it does |
|---|---|
| `Instance`, `InstanceSource`, `YAMLSource` | The instance model and where instances come from. |
| `Directory` | Instances by hostname and id, kept current from the source. |
| `Handler` | Health path, peer check, hostname, blocked, then remote (the `Forward` hook) or local (the runtime), with the gates. |
| `Runtime` | What a provider implements. Exactly one per process. |
| `Gate`, `RegisterSectionGate`, `ChainGates` | Request gates. |
| `Serve`, `TLSProvider` | Listeners: plain, TLS (a provider such as autotls, or cert files reloaded on change), redirect, graceful shutdown. |
| `Host`, `FromConfig` | All of the above assembled, by hand or from `pocketone.yaml`. |

## License

MIT
