# Auto TLS

Automatic HTTPS certificates through ACME, with [CertMagic](https://github.com/caddyserver/certmagic) (Caddy's
certificate engine). PocketBase already gets Let's Encrypt certificates for `serve example.com`. Auto TLS takes over
that job and adds other certificate authorities, extra domains and wildcard certificates.

Status: stable under pocker, beta on a single app.

## Install

Auto TLS is built into `pocketone` and **off by default**. Turn it on with `--autotls` (or `auto: true` in
`pocketone.yaml`). It only does something when the app serves HTTPS, that is with `serve <domain>` or `--https`; on
plain HTTP it stays idle, and without it PocketBase's own certificates are used as before.

In your own Go build, register it before the app starts:

```go
import (
	"github.com/pockethost/pocketone/ext/autotls"
	"github.com/pockethost/pocketone/ext/pluginkit"
)

app := pocketbase.New()
pluginkit.MustRegister(app, autotls.NewApp(autotls.Config{Auto: true, Email: "ops@example.com"}))
app.Start()
```

## Configure

### Command-line flags

```sh
pocketone serve example.com --autotls --autotlsEmail ops@example.com
pocketone serve example.com --autotls --autotlsDomains "*.example.com" --autotlsDNS cloudflare   # token in $CF_API_TOKEN
```

| Flag | Default | What it does |
|---|---|---|
| `--autotls` | `false` | Turn it on. |
| `--autotlsEmail` | | ACME account email, for expiry notices from the CA. |
| `--autotlsCA` | `letsencrypt` | `letsencrypt`, `letsencrypt-staging`, `zerossl`, or an ACME directory URL. |
| `--autotlsDomains` | | Extra domains, obtained at start (comma separated). Wildcards need `--autotlsDNS`. |
| `--autotlsDNS` | | DNS provider for DNS-01 challenges. Only `cloudflare` is supported. |
| `--autotlsDNSTokenEnv` | `CF_API_TOKEN` | Environment variable holding the DNS provider's API token. |

### pocketone.yaml

The `autotls` section sets the defaults for those flags. Flags win over the file.

```yaml
autotls:
  auto: true
  email: ops@example.com
  ca: letsencrypt
  domains: ["*.example.com"]   # wildcards need dns
  dns:
    provider: cloudflare
    api_token: $CF_API_TOKEN   # read from the .env beside pocketone.yaml
```

`storage` is only in the file: where certificates are kept (default `pb_data/.autotls`). The DNS token can be given
as `api_token` (write `$VAR` to read it from the `.env` beside the file) or through `api_token_env`, the name of an
environment variable. `api_token` is only used when `--autotlsDNS` names the same provider as the file.

The Cloudflare token needs permission to edit the zone's DNS.

There are no dashboard settings.

## Use

Once on, there's nothing to click. Start the app as usual with `serve <domain>` (or `--https`):

- **Which hostnames get a certificate.** The domains given to `serve` and their `www.` variants, issued on the first
  request for each, plus `--autotlsDomains`, obtained at start. Any other hostname is refused at the TLS handshake, so
  a random `Host` can't trigger issuance.
- **How they're proved.** TLS-ALPN-01 on the HTTPS port, or DNS-01 when a DNS provider is set (needed for wildcards).
  HTTP-01 isn't used on a single app, because port 80 is PocketBase's redirect server.
- **Renewal** is automatic.
- **Where they're stored.** `pb_data/.autotls` by default, separate from PocketBase's own `.autocert_cache`.

Settings are checked at start, before anything touches the network. A wildcard domain without a DNS provider, a
Cloudflare provider without a token, or an unknown provider stops the app with an error such as:

```
tls.domains: wildcard "*.example.com" needs a DNS provider (tls.dns); per-hostname certificates work without one
```

With debug logging, the start shows `autotls: CertMagic serves HTTPS certificates` with the CA and extra domains.

Use `--autotlsCA letsencrypt-staging` while testing, so failed attempts don't count against Let's Encrypt's rate limits.

## Integrations

### Pocker

Under [pocker](../pocker/README.md), the stock `pocketone` binary (with `provider: pocker`) runs `autotls.New()`, a
server plugin that gets certificates for every instance the host serves. It's configured by the same `autotls`
section:

```yaml
autotls:
  auto: true
  email: ops@example.com
  storage: ./pb_root/.pocker-certs   # default: .pocker-certs in the pocker root
  domains: ["*.example.com", "example.com"]
  dns: { provider: cloudflare, api_token_env: CF_API_TOKEN }   # only needed for wildcards
```

- Without `dns`, every hostname the instance source maps gets its own certificate on its first TLS handshake, with any
  DNS host and no API token. Hostnames no instance serves never trigger issuance.
- Challenges include HTTP-01 on the pocker server's port 80, as well as TLS-ALPN-01 and DNS-01.
- Certificate files (`server.tls.cert`/`key`) stay a core pocker feature. Setting both files and `auto` is an error.
- Certificates belong to the pocker server, not to instances; the single-app plugin (`NewApp`) is a separate type so
  it never runs per instance.

### Building on Auto TLS

- `autotls.New()` implements pocker's `Configurer` and `TLSProvider`. A host that builds its own `pocker.Server` adds
  it to `Plugins`:

  ```go
  srv := &pocker.Server{Config: cfg, Plugins: []pocker.Plugin{autotls.New()}}
  ```

- `AutoTLS.Config` can be set in Go before the server starts; the `autotls` section is read only when `Auto` is false
  there.
- `autotls.SupportedDNSProviders` lists the `dns.provider` values it understands.

## How it works

- Go module `github.com/pockethost/pocketone/ext/autotls`.
- On a single app, an `OnServe` hook swaps the TLS config of PocketBase's HTTPS server for CertMagic's. PocketBase's
  own host policy (the `serve` domains and `www.`) decides which on-demand hostnames are allowed. The certificate
  manager stops when the app terminates.
- TLS 1.2 is the minimum; `h2` and `http/1.1` are offered, and `acme-tls/1` stays on for TLS-ALPN challenges.
- A wildcard in `domains` covers one level (`*.example.com` covers `a.example.com`, not `a.b.example.com`).

## License

MIT
