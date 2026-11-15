# Bulk Mail

Campaigns to your app's own users, from the dashboard. You pick who gets the mail with a collection, a view or SQL,
write the message, and Bulk Mail sends it at a steady pace through your app's mail settings. It adds one-click
unsubscribe, keeps a list of addresses that must never be mailed again, and handles bounces and spam complaints.

## Install

Bulk Mail is built into `pocketone` and **on** for every app. Nothing is sent until a superuser starts a campaign.

In your own Go build, register the extension before the app starts. Bind it into the JS runtime too if you want
`$bulkmail` in `pb_hooks`:

```go
mail := bulkmail.New(bulkmail.Config{Rate: 5, Daily: 5000})

app := pocketbase.New()
extkit.MustRegister(app, mail)
jsvm.MustRegister(app, jsvm.Config{OnInit: func(vm *goja.Runtime) { extkit.BindJS(app, vm, mail) }})
app.Start()
```

The module is `github.com/pockethost/pocketone/ext/bulkmail`.

## Configure

There is no `pocketone.yaml` section or command-line flag. The limits are fixed in the binary; `pocketone` uses the
defaults:

| Limit | Default | What it does |
|---|---|---|
| Per second | 5 | The most messages this app sends per second. A campaign can only lower it. |
| Per day | 5,000 | The most messages this app sends per UTC day, across all campaigns. |

In a custom build, set them with `bulkmail.Config{Rate, Daily}`. `Daily: bulkmail.NoBulkMail` forbids sending; the
screens then say so.

### Settings → Mail

Campaigns send through the app's own mail settings and from its sender, both set on **Settings → Mail**. Below that
form there's a **Bulk mail** section:

- **Postal address**: shown in every campaign footer. Anti-spam laws require one.
- **Unsubscribe mailbox**: optional. Adds a `mailto:` alternative to the `List-Unsubscribe` header.
- **Bounces and complaints** (click to open):
  - **Amazon SES (SNS) endpoint**: the URL to subscribe to the SNS topic your SES identity sends bounce and complaint
    notifications to.
  - **Allowed SNS topic ARNs**: comma separated. Empty accepts any topic with a valid AWS signature.
  - **Generic webhook** and **Webhook secret**: for other providers. The webhook is off until you set a secret.

The section also shows this app's limits and how many messages it sent today. Click **Save bulk mail settings**.

Unsubscribe links and the feedback URLs are built from **Settings → Application → Application URL**, so set it to
the address people reach your app on.

The settings are stored in the app's database (a `_params` row), so they go along with its backups. Secrets are
never shown again after saving.

## Use

The dashboard gets a **Mail** link in its menu, with two pages: **Campaigns** and **Suppressions**. Only superusers see
them.

### Write a campaign

1. Open **Mail** and click **New campaign**. It starts as a draft for verified users of `users`.
2. Fill in **Name**, **Subject** and **Body**.
3. Under **Audience**, choose who gets it:
   - **Filter**: a **Collection** (or a view), an optional **Filter** in the same syntax as API rules, and the
     **Email field** that holds the address (an email or text field, or a view's computed column).
   - **SQL**: a read-only `SELECT` that returns an `email` column. Every other column is available to the message.

   Click **Count recipients** to see how many match, how many are suppressed, and a few sample addresses.
4. Optional: **Vars (JSON)** for values used in the message, a **Schedule (UTC)**, and **Max per second** to send
   slower than the app's limit.
5. Click **Save changes**. **Preview** shows the message for one recipient; **Send test** mails it to the
   **Test address**.

A view is the easy way to build a dynamic audience: it picks who gets mail and computes the columns the message
uses. The [example](example/demo.go) creates one called `newsletter_audience`.

### Placeholders

In the subject and body:

- `{{record.field}}`: a field or column of the recipient's row, for example `{{record.name}}`.
- `{{email}}`, `{{campaign.name}}` and `{{unsubscribe_url}}`.
- Campaign vars: `{"promo": "X"}` gives `{{promo}}`. A var can be computed once when sending starts:
  `{"count": {"sql": "SELECT COUNT(*) AS value FROM users"}}`.
- `{{if record.field}}…{{else}}…{{end}}` blocks, and `{{if !record.field}}` for the opposite.

Values are HTML-escaped in the body. An unsubscribe footer with your postal address is always added.

### Send

Click **Send** (or **Schedule** when a time is set) and confirm. The campaign page shows its status and progress
(sent, queued, skipped, failed), and a **Recipients** table you can filter by status. While it runs you can
**Pause**, **Resume** or **Cancel** it.

- Every address is queued once per campaign. Sending a finished campaign again mails only new matches.
- A message counts as sent only once the mail server accepts it, so a restart doesn't send twice or skip anyone.
- A permanent error (SMTP 5xx) fails that recipient. Other errors are retried, up to three attempts.
- A campaign pauses itself when more than 5% of its sends fail after the first 100, or when spam complaints pass
  0.3% of its sends (at least 3). The reason shows next to its status.

### Suppressions

**Mail → Suppressions** lists the addresses that never receive campaign mail, whatever the audience says.
Unsubscribes, permanent bounces and complaints are added automatically. Click **Add address** to add one by hand, a
row to edit or delete it, or **Export CSV** to download the list.

### Unsubscribing

Every message has a signed unsubscribe link and the `List-Unsubscribe` and `List-Unsubscribe-Post: One-Click`
headers that Gmail and Yahoo require for bulk senders. Opening the link shows a confirm button, so link scanners
can't unsubscribe anyone. The one-click request from a mail provider unsubscribes directly.

### Try it locally

In dev mode with SMTP off, `pocketone` catches every mail, bulk mail included, in an inbox at
http://127.0.0.1:8025:

```bash
pocketone serve --dev
```

## Integrations

### Works with

| Extension | How they fit |
|---|---|
| [Pocker](../pocker/README.md) | Add the same extension value to `srv.Extensions` and every instance gets Bulk Mail. Each instance can override its limits (below). `$bulkmail` only reaches the instance's own data, so it's safe for untrusted tenants. |
| [Mailtrap](../mailtrap/README.md) | In dev mode with SMTP off, campaign mail lands in the local inbox instead of being sent. |

### Per-instance settings (pocker)

Each instance's `extensions.bulkmail` settings apply over the `Config` given to `bulkmail.New`:

```yaml
instances:
  - id: abc
    extensions: { bulkmail: { rate: 2, daily: 1000 } }
  - id: free-plan-app
    extensions: { bulkmail: { daily: 0 } }      # may not send bulk mail
  - id: other
    extensions: { bulkmail: { enabled: false } } # no Bulk Mail at all
```

### Go API

`bulkmail.For(app)` returns the app's service: `Start`, `StartLimit`, `Pause`, `Resume`, `Cancel`, `Preview`, `Test`,
`CountAudience`, `Suppress`, `Unsuppress`, `IsSuppressed`, `Unsubscribe`, `UnsubscribeURL`, `Limits`, and the hooks:

- `OnSend` fires before every campaign message (not test sends). Change the message, or return an error to fail that
  recipient.
- `OnUnsubscribe` fires when someone unsubscribes through a link or the one-click header.
- `OnFeedback` fires for each bounce or complaint a provider reports.

`Config.Mailer` swaps the mail client, for example for a platform relay. `Config.PublicURL` sets the base of
unsubscribe links instead of the Application URL.

### JS API

`$bulkmail` in `pb_hooks`:

```js
$bulkmail.start(id, [limit])     // limit: queue only the next N new recipients
$bulkmail.pause(id); $bulkmail.resume(id); $bulkmail.cancel(id)
$bulkmail.test(id, [to]); $bulkmail.preview(id, [email])
$bulkmail.count({collection, filter, email_field} | {sql})   // {count, suppressed, sample}
$bulkmail.suppress(email, [reason]); $bulkmail.unsuppress(email); $bulkmail.isSuppressed(email)
$bulkmail.unsubscribe(email, [campaignId]); $bulkmail.unsubscribeURL(email, [campaignId])
$bulkmail.limits()               // {rate, daily, sentToday}
```

React to activity with record hooks on the collections below, for example
`onRecordAfterCreateSuccess((e) => { … }, "_bulkmail_suppressions")`.

### REST API

Superusers only:

| Route | What it does |
|---|---|
| `GET`/`PATCH /api/bulkmail/settings` | Settings (secrets are never returned), limits and feedback endpoint URLs. |
| `POST /api/bulkmail/audience/count` | `{"audience": {...}}` → count, suppressed, sample. |
| `POST /api/bulkmail/campaigns/{id}/start\|pause\|resume\|cancel` | Campaign actions. `start` takes an optional `{"limit": N}` to queue only the next N new recipients. |
| `POST /api/bulkmail/campaigns/{id}/preview` | The rendered message for one recipient. |
| `POST /api/bulkmail/campaigns/{id}/test` | Send to `test_to` (no ledger rows). |

Campaigns and suppressions are created and edited through the regular records API. A campaign's `from_name` and
`from_address` override the app's sender, but only through the records API, JS or Go (the dashboard doesn't show
them), since the app's mail account can usually send only as identities it has verified.

Public routes: `GET`/`POST /api/bulkmail/unsubscribe?t=…`, `POST /api/bulkmail/feedback/ses` and
`POST /api/bulkmail/feedback/webhook`. The webhook takes `{"type": "bounce|complaint|unsubscribe", "email": "…"}`
with an `X-Bulkmail-Secret` header.

## Example

[`example/main.go`](example/main.go) runs one app with bulk mail: the Mail page, campaigns to the app's own users and `$bulkmail` in JS hooks. Run it with `go run ./example serve --dev` from this folder; the dev mail trap at http://127.0.0.1:8025 catches every mail.

```go
// Command example runs one PocketOne app with bulk mail: a "Mail" page in the dashboard, campaigns to the app's
// own users, and $bulkmail in JS hooks.
//
// Run it in dev mode: while SMTP is off, PocketOne's dev mail trap catches every mail (bulk mail included) and
// shows it at http://127.0.0.1:8025, with nothing else to install:
//
//	go run ./example serve --dev
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password, app user user@example.com / password.
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/plugins/jsvm"
	"github.com/dop251/goja"
	"github.com/pockethost/pocketone/ext/bulkmail"
	"github.com/pockethost/pocketone/ext/devusers"
	"github.com/pockethost/pocketone/ext/extkit"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"

	registerDemoView(app) // a "newsletter_audience" view to use as a campaign audience (demo.go)

	// the same extension value pocker would host on every instance (srv.Extensions = append(srv.Extensions, mail))
	mail := bulkmail.New(bulkmail.Config{
		Rate:  5,    // messages per second
		Daily: 5000, // messages per day; bulkmail.NoBulkMail forbids sending
	})
	extkit.MustRegister(app, mail)

	jsvm.MustRegister(app, jsvm.Config{
		OnInit: func(vm *goja.Runtime) { extkit.BindJS(app, vm, mail) }, // $bulkmail in JS hooks
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## How it works

- Go module `github.com/pockethost/pocketone/ext/bulkmail`, an [`extkit`](../extkit/extkit.go) extension.
  [`example/main.go`](example/main.go) runs one app with Bulk Mail (`go run ./example serve --dev`).
- The audience is resolved when sending starts and queued into a ledger first (one row per campaign and address,
  unique). A background sender then mails one message at a time through the app's mail client, at the lower of the
  app's rate and the campaign's.
- SQL audiences and SQL vars run with SQLite's `query_only` on, inside a transaction that is always rolled back.
- Unsubscribe links are signed with HMAC and a per-app secret, generated on first use.
- SES feedback: SNS signatures are verified, and signing certificates are fetched only from
  `sns.<region>.amazonaws.com`. Permanent bounces and complaints go on the suppression list.

### Data

| Collection | What |
|---|---|
| `_bulkmail_campaigns` | Content, audience, vars, status, counters, schedule. |
| `_bulkmail_sends` | The ledger: campaign, email, status (`queued`, `sent`, `skipped`, `failed`), reason, attempts. |
| `_bulkmail_suppressions` | Addresses that never receive campaign mail, with the reason. |

All are superuser-only system collections with fixed collection and field ids ([`schema.go`](schema.go)).
`LegacyJSMigration()` emits a PocketBase JS migration that creates the same collections, for apps that move onto this
schema before they run this extension.

## License

MIT
