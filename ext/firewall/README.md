---
owners: ["@benallfree"]
version: 0.0.1
title: "Firewall"
group: many
summary: "Peer allow-lists, trusted proxies and rate limits for a host."
---

# Firewall

Peer allow-lists, trusted proxies and client IPs, and per-IP and per-instance rate limits for a
[Multisite](../multisite/README.md), with or without [Edge](../edge/README.md). Every request is checked before an instance
serves it or it's forwarded to another edge.

## Install

The firewall is built into `pocketone`. It turns on in host mode (a `multisite` section in `pocketone.yaml`), as the host's [gate/v1](../contracts/gate/v1/README.md), when the file has
a `firewall` or `rate_limits` section; there's nothing else to install.

In your own Go build, importing the package registers it with the host, with the same rule:

```go
import _ "github.com/pockethost/pocketone/ext/firewall"
```

Or build one in Go and add it to the host's gates, as [the example](#example) does.

A single app doesn't need it: stock PocketBase has its own per-IP rate limits and trusted proxies.

## Configure

Two sections, both optional; either one turns the firewall on (keys below).

```yaml
firewall:
  allow_cidrs: []                 # peers allowed to connect (empty = everyone)
  trusted_proxies: []             # peers whose client-IP headers are believed
  client_ip_headers: [CF-Connecting-IP, True-Client-IP, X-Forwarded-For, X-Real-IP]
  trusted_clients: []             # client IPs that get the trusted rate limits on every instance
  trusted_client_headers: [X-Client-IP] # a trusted client names its end user: each user gets their own limits
  rate_limit_header_prefix: X-RateLimit
rate_limits:                      # defaults for every instance
  ip_hourly: 1000
  instance_hourly: 10000
  ip_concurrent: 15
  instance_concurrent: 250
  file_weight: 0.1                # a /api/files request costs a tenth
  trusted: { ip_hourly: 5000, instance_hourly: 20000, ip_concurrent: 50, instance_concurrent: 250 }
  # off: true                     # no rate limiting
```

The values shown are the defaults. In dev mode (`instances: { dev: true }`) rate limiting is off unless the file has a
`rate_limits` section. A trusted limit is never lower than the normal one.

Each instance can set its own limits and trusted IPs in its `extensions.firewall` settings; missing fields keep the
defaults:

```yaml
instances:
  - id: api
    hosts: [api.example.com]
    extensions:
      firewall: { rate_limits: { ip_hourly: 5000, instance_concurrent: 500 }, trusted_ips: [203.0.113.0/24] }
```

## Use

What callers see:

- **Peers outside `allow_cidrs`** get 403.
- **Client IP:** taken from `client_ip_headers` only when the peer is a trusted proxy; otherwise it's the peer. The
  app then sees that client as the request's IP, so PocketBase's logs and its own rate limits see the visitor.
- **Over a limit:** 429 with `Retry-After` and a message naming the limit and the IP.
- **Every routed response** carries `X-RateLimit-{Ip,Instance}-{Hourly,Concurrent}-{Limit,Remaining}` and
  `X-RateLimit-{Ip,Instance}-Hourly-Reset` (with your prefix), listed in `Access-Control-Expose-Headers` so browser
  code can read them.
- **Long-lived responses** (realtime) count toward the concurrency limits until they end.

### Trusted clients that serve many users

A trusted client (process-wide `trusted_clients`, or an instance's own `trusted_ips`) that serves many users, such as
a server-side proxy or SSR app, sends each request's end-user IP in a `trusted_client_headers` header. Per-IP limits
then apply to that user, still with the trusted limits; without the header all its users share one bucket. The header
is ignored from anyone else, so it can't be spoofed to escape a limit.

## Integrations

### Works with

| Extension | How they fit |
|---|---|
| [Multisite](../multisite/README.md) | The firewall is a host gate: it checks the peer, then admits each request before the runtime or the forward hook. |
| [Pocker](../pocker/README.md), [Docker](../docker/README.md) | Either provider; the firewall runs in front of both. |
| [Edge](../edge/README.md) | The entry edge applies the limits before forwarding. The owner edge's rate-limit headers replace the entry edge's, since the owner counted every request for the instance. |
| [Multitenant](../tenants/README.md) (planned) | Will write each instance's `extensions.firewall` settings, such as its owner's trusted IPs. |

### Building on the firewall

- **Per instance.** Write `extensions.firewall.rate_limits` and `extensions.firewall.trusted_ips` into an instance's
  settings (from the instances file or your own instance source).
- **Go API.** `firewall.Firewall{Config, RateLimits}` is a `multisite.Gate` you can add to `Host.Gates` yourself;
  `NewFirewall(cfg)` builds one ready to use. `ClientIP(r)` gives a request's client. `firewall.SectionAdjust` changes
  the settings read from `pocketone.yaml` before the firewall is built.

## Example

The firewall in front of a toy host: only `127.0.0.1` may connect, at 100 requests an hour per IP. Run it from this
folder with `go run ./example --instances instances.yaml`.

```go
// The firewall in front of a host: every request is checked (allowed peers, rate limits) before a toy runtime
// answers. In a PocketOne binary the firewall turns on from pocketone.yaml's firewall / rate_limits sections; here
// it is built in Go.
//
//	go run ./example --instances instances.yaml
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pockethost/pocketone/ext/multisite"
	"github.com/pockethost/pocketone/ext/firewall"
)

type okRuntime struct{}

func (okRuntime) Name() string                        { return "ok" }
func (okRuntime) Serves(inst multisite.Instance) bool { return true }
func (okRuntime) Ready() bool                         { return true }
func (okRuntime) ServeInstance(w http.ResponseWriter, _ *http.Request, inst multisite.Instance) {
	fmt.Fprintf(w, "hello from %s\n", inst.ID)
}

func main() {
	instances := flag.String("instances", "instances.yaml", "instances file")
	listen := flag.String("listen", "127.0.0.1:8090", "listen address")
	flag.Parse()
	fw := &firewall.Firewall{
		Config:     firewall.Config{AllowCIDRs: []string{"127.0.0.1"}},
		RateLimits: &firewall.RateLimits{IPHourly: 100},
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	h := &multisite.Host{Source: &multisite.YAMLSource{Path: *instances}, Runtimes: []multisite.Runtime{okRuntime{}}, Gates: []multisite.Gate{fw}, Listeners: multisite.Listeners{Listen: *listen}}
	if err := h.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

## License

MIT
