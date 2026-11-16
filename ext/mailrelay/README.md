---
owners: ["@benallfree"]
version: 0.0.1
title: "Mail Relay"
group: services
summary: "Outgoing mail for instances without SMTP, through your relay, with limits."
---

# Mail Relay

Outgoing mail for [Pocker](../pocker/README.md) instances that haven't set up SMTP.

Stock PocketBase falls back to the host's `sendmail` when SMTP is off. Under Pocker that would run `sendmail` on the
shared host for every tenant. With this extension:

- **SMTP on** in an instance's Settings → Mail: the app sends the mail itself. Bring your own SMTP always wins.
- **SMTP off:** the mail goes through your relay as `<instance>@<Domain>`, within per-instance limits. `sendmail`
  never runs.

## Install

Mail Relay is not in the `pocketone` binary. Add it to the extensions of your own Pocker build, with your sending
domain and relay:

```go
srv.Extensions = append(srv.Extensions, mailrelay.New(mailrelay.Config{
    Domain:   "mail.example.com",
    Upstream: mailrelay.SMTPUpstream{Addr: "smtp.provider.com:587", Username: "…", Password: "…"},
}))
```

The module is `github.com/pockethost/pocketone/ext/mailrelay`. It works the same in `pocker.Provider{Extensions: ...}`.
It also registers on a single app, but a single app can set up its own SMTP and doesn't need a relay.

## Configure

### The relay (Go)

`Domain` and `Upstream` are required; the host doesn't start without them.

- **`Domain`**: every relayed message is From `<local part>@Domain`.
- **`Upstream`**: what delivers the mail. `SMTPUpstream` ships here: `Addr` (host:port), optional `Username` and
  `Password` (PLAIN auth, only over TLS or to localhost), `ImplicitTLS` for port 465 (otherwise STARTTLS when the
  server offers it), and `Timeout` (default 30 s). It dials with its own dialer, so Pocker's egress rules for tenants
  (public addresses only) don't block a relay on a private address. Anything with
  `Send(ctx, from, to, raw)` works, for example an Amazon SES `SendRawEmail` sender.
- **`LocalPart`**: picks an instance's address from its id. The default is the first label of the id, lowercased:
  `blog.localhost` → `blog`.
- **Limits:** `PerMinute` (default 20) and `Daily` (default 200) per instance, `MaxRecipients` (To + Cc + Bcc,
  default 5) and `MaxSize` (encoded message, default 1 MiB) per message.

### Defaults for every instance (`pocketone.yaml`)

```yaml
mailrelay: { per_minute: 20, daily: 200 }
```

Limits set in Go win over this section. Each instance can override it (see [Per-instance settings](#per-instance-settings-pocker)).

## Use

Once it's in, there's nothing to run. Each instance's mail is relayed whenever its SMTP is off.

- **Settings → Mail** in an instance's dashboard shows its superusers the relay: the address it sends as, its limits
  and how many messages it sent today. When sending is off for the instance, it says so and points to the SMTP
  settings.
- **The app's log** (Admin > Logs, which Pocker copies to the instance's own log) gets one line per message, with
  masked addresses: `mailrelay: sent from=blog@… to=a***@x.org`. Refusals and upstream failures are warnings.
- **A refused message** returns an error to whoever triggered the mail, such as a password-reset request: sending
  off, too many recipients, too large, or the limit reached.

## Integrations

### Works with

| Extension | How they fit |
|---|---|
| [Contracts](../contracts/README.md) | Provides `mail.fallback/v1`: with SMTP off, an app's mail goes to the contract's providers in order (`providers: { mail.fallback/v1: [mailtrap, mailrelay] }`); Mail Relay sends it within the instance's limits. |
| [Pocker](../pocker/README.md) | The host it's built for. Every instance gets the relay, its own address and its own limits. |
| [Mail Trap](../mailtrap/README.md) | In development, point `SMTPUpstream` at the trap's SMTP port and every relayed message lands in one inbox, tagged with the instance. The example below does this. |
| [Bulk Mail](../bulkmail/README.md) | Campaigns go through PocketBase's mailer by default, so with SMTP off they are relayed and count against the instance's limits. |

### Per-instance settings (pocker)

Each instance can override the defaults in its `extensions.mailrelay` settings, from `instances.yaml` or your instance
source (for example, per plan):

```yaml
instances:
  - id: free-plan-app
    extensions: { mailrelay: { send: false } }               # refuse its mail unless it uses its own SMTP
  - id: big-app
    extensions: { mailrelay: { per_minute: 60, daily: 2000 } }
```

A change to these settings restarts the instance's app on its next request.

Use `send: false`, not `enabled: false`, to stop an instance sending. `enabled: false` turns the extension off for that
instance, and with SMTP off PocketBase then falls back to the host's `sendmail`. Don't set it under Pocker.

### Building on Mail Relay

- **Your own upstream.** Implement `mailrelay.Upstream` (`Send(ctx, from, to, raw)`) to deliver through an API.
- **Your own hooks first.** The relay runs after the app's own `onMailerSend` hooks, so they can still change or
  drop a message.
- **Go API.** `(*mailrelay.Extension).Address(id)` returns the From address an instance sends as.
- **settings/v1** (superusers): `GET /api/_c/settings/v1/get?extension=mailrelay` returns the instance's relay status, a read-only section: `active`, `send`, `address`,
  `perMinute`, `daily`, `sentToday`.

## Example

[`example/main.go`](example/main.go) runs a Pocker host whose relay sends to the dev mail trap, so every instance's
mail lands in one inbox at http://127.0.0.1:8025. Run `go run ./example --dev` from this folder, then trigger a
password reset on any instance:

```bash
curl -X POST http://alpha.localhost:8090/api/collections/users/request-password-reset -d email=user@example.com
```

Log in to an instance's dashboard as `admin@example.com` / `password` to see the relay on Settings → Mail. `--listen`
changes the instances' port; `P1_MAILTRAP_HTTP` and `P1_MAILTRAP_SMTP` change the trap's.

```go
// Command example is a Pocker build with mailrelay, relaying into PocketOne's dev mail trap: every instance
// without its own SMTP sends as <instance>@mail.localhost, and all of it lands in one inbox.
//
//	go run ./example --dev
//
// Then trigger mail on any instance, for example a password reset (each instance has the test user below):
//
//	curl -X POST http://alpha.localhost:8090/api/collections/users/request-password-reset -d email=user@example.com
//	curl -X POST http://beta.localhost:8090/api/collections/users/request-password-reset -d email=user@example.com
//
// and read it in PocketOne's dev mail trap at http://127.0.0.1:8025 (filter by instance; P1_MAILTRAP_HTTP and
// P1_MAILTRAP_SMTP change its ports). Settings → Mail in each dashboard shows the relay.
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password, app user user@example.com / password.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/devusers"
	"github.com/pockethost/pocketone/ext/mailrelay"
	"github.com/pockethost/pocketone/ext/mailtrap"
	"github.com/pockethost/pocketone/ext/pocker"
	"github.com/pockethost/pocketone/ext/standard"
)

func main() {
	dev := flag.Bool("dev", false, "serve any hostname (alpha.localhost, beta.localhost, ...); otherwise instances.yaml is required")
	listen := flag.String("listen", "", "HTTP address for the instances (default :8090)")
	flag.Parse()

	cfg := pocker.DefaultConfig()
	cfg.Dev = cfg.Dev || *dev
	if *listen != "" {
		cfg.Listen = *listen
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// The dev mail trap (ext/mailtrap) stands in for the operator's real relay (SES, a provider's SMTP, ...)
	inbox, err := mailtrap.Default("mailrelay example: the relay sends to the trap")
	if err != nil {
		log.Fatal(err)
	}

	srv := &pocker.Server{
		Config:   cfg,
		Standard: standard.New(), // every instance gets the standard extensions
		Extensions: []pocker.Extension{
			mailrelay.New(mailrelay.Config{
				Domain:   "mail.localhost",
				Upstream: mailrelay.SMTPUpstream{Addr: inbox.SMTPAddr},
			}),
		},
		// local testing: every instance gets superuser admin@example.com and user user@example.com, password "password"
		SetupApp: func(_ pocker.Instance, app core.App) error { return devusers.Ensure(app) },
	}
	if err := srv.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
```

## How it works

- The extension binds PocketBase's `OnMailerSend` on every instance, last, after the app's own hooks. No core change.
- When SMTP is off it doesn't call `e.Next()`, so PocketBase's `sendmail` fallback never runs. It relays the message
  in-process instead.
- Nothing is written to the instance's settings. A backup restored somewhere else has no relay credentials in it, and
  falls back to stock PocketBase behavior.

For each message:

1. **Check it's allowed.** May this instance send (`send`)? Are there no more than 5 recipients (To + Cc + Bcc)? Is
   the encoded size no more than 1 MiB?
2. **Count it.** Each message counts once against the instance's limits, however many recipients it has. The day
   resets at midnight UTC. Counts are kept in memory.
3. **Set the sender.** The From address becomes `<local part>@<Domain>`. The display name is kept, minus anything
   that looks like an address or a link. The app's own sender address becomes `Reply-To`, unless it's PocketBase's
   `@example.com` placeholder.
4. **Build and send.** Bcc stays out of the headers. An `X-Pocker-Instance` header is added, then the message goes to
   the `Upstream`.
5. **Log it** to the app's log.

## License

MIT
