# OAuth Server

Makes your app an OAuth 2.1 authorization server whose access tokens are [API tokens](../tokens/README.md).
`pocketone login` signs in to it in the browser (or with a code on another device), and MCP clients such as Claude
or ChatGPT connectors register themselves and connect. Superusers and app users sign in on your app's own pages,
through PocketBase's own auth endpoints.

## Install

OAuth Server is built into `pocketone` and always on, with [Tokens](../tokens/README.md). There's nothing to install
or turn on.

In your own Go build, register Tokens and OAuth Server before the app starts:

```go
app := pocketbase.New()
tokens.MustRegister(app, tokens.Config{})                // required: access tokens are API tokens
remotes.MustRegister(app, remotes.Config{})              // optional: lists "oauth" in /_pocker/whoami so login finds it
oauthserver.MustRegister(app, oauthserver.Config{})
app.Start()
```

Without Tokens the app refuses to start ("oauth-server needs ext/tokens"). The module is
`github.com/pockethost/pocketone/ext/oauth-server`.

## Configure

There is no `pocketone.yaml` section, flag or environment variable for OAuth Server. In `pocketone`:
- superusers and every auth collection may sign in;
- access tokens last 30 days, refresh tokens 180 days (renewed on each refresh);
- tokens for [MCP](../mcp/README.md) never expire, since MCP turns that on for `/mcp`;
- client self-registration (`POST /oauth/register`) is on, since MCP turns it on;
- Claude and Claude Code connect without registering (their client metadata documents from `claude.ai` and
  `claude.com` are trusted).

A Go build can change these (see [Go config](#go-config)).

### Sign-in methods

The pages offer exactly the methods each auth collection has on, so you set them where you always do: the
collection's options in the dashboard (**Collections →** the collection **→ Options**).
- **Password**, **One-time password** (an emailed code) and **OAuth2** providers each get their own form or button.
- **Multi-factor authentication** adds a second step.
- The collection's auth rule (for example `verified = true`) applies.

OAuth2 providers (Google, GitHub, …) are offered to app users only; PocketBase doesn't allow them for superusers.
**Register `<your app's URL>/oauth/provider/callback` as a redirect URI with each provider.**

The pages show **Settings → Application → Application name**.

### Behind a proxy

The server names itself (the issuer) after the scheme and host the browser used, with `https` when the request is TLS
or `X-Forwarded-Proto` says so. If a proxy rewrites the host, set `Config.Issuer` in a Go build so the issuer and the
provider callback URL stay stable.

### Scopes

A client may ask for any scope registered with [Tokens](../tokens/README.md#scopes), including your own from
`$tokens.registerScope`, and the consent page lists them with their descriptions. No scope means the account's full
rights, and the consent page says **Full access**. `offline_access` is accepted (refresh tokens are always issued).

### Your own sign-in pages

The built-in pages (sign-in, second step, one-time code, consent, device code, done, error) follow the system's light
or dark mode and work on phones. To change them, eject them as [PocketPages](../pages/README.md) templates:

```bash
pocketone oauth eject           # --force overwrites files you already have
```

It writes `signin.ejs`, `mfa.ejs`, `otp.ejs`, `consent.ejs`, `device.ejs`, `done.ejs`, `error.ejs` and the shared
`head.ejs`, `foot.ejs` and `hidden.ejs` (with the CSS) to `pb_pages/_private/oauth/`, and prints each file. From then
on the server renders your templates; a missing one falls back to the built-in page, so delete them to go back.
`_private` keeps them from being served as routes. See [Custom page templates](#custom-page-templates) for the data
they get.

## Use

### Signing in

Whenever a client sends someone to your app (`pocketone login`, Claude, an app of yours), they see:

1. **Sign in to <app name>**: the form for app users (`users` first; with several user collections, a tab for
   each). At the bottom, **Sign in as a superuser** switches to the superuser form, which looks clearly different:
   a **Superuser** badge, **Superuser sign-in**, its own colors and a warning. **Sign in as a user instead** goes back.
2. **One more step** when the account has MFA, offering its other methods. After a password, when an emailed code is
   the only method left, it's sent right away.
3. **Check your email** for an emailed code.
4. The consent page: the client's name, the account, whether the client will act as a user (and which collection) or
   as a superuser, and the scopes. Click **Allow** or **Deny**, or **Not <email>? Use another account**.

With no user collection allowed, the superuser form is the only one. A browser stays signed in to these pages for 15
minutes. Failed attempts are limited to 10 per IP in 5 minutes.

On another device (over SSH, say), the client shows a code like `WDJB-MJHT`. Open
`https://<your app>/oauth/device/verify`, enter it under **Connect a device**, sign in and allow. Codes last 10 minutes.

### Log in from the command line

```bash
pocketone login prod
```

With OAuth Server on the remote, [Remotes](../remotes/README.md) opens your browser on the pages above, or shows a
device code when there's no browser (`--device` asks for one). The client is the built-in `pocketbase-cli`, named
**pocketone CLI** on the consent page (the name follows the command, so another build shows its own).

### See and revoke what's connected

Every sign-in creates an API token named "<client> (OAuth)". They're listed under **Settings → API tokens** with an
`oauth: <client id>` label; click **Revoke** to disconnect a client. Revoking the access token also ends its refresh
token. A client can revoke its own tokens at `POST /oauth/revoke`, which `pocketone logout` does.

### Register clients

Claude and the CLI need no registration, and MCP clients register themselves. For your own app:

```bash
pocketone oauth clients add --name "My app" --redirect https://myapp.example.com/callback
pocketone oauth clients add --name "Desktop" --redirect http://127.0.0.1/callback --public
pocketone oauth clients list
pocketone oauth clients rm <client_id>
```

`add` prints the client id, and the secret once for a confidential client. `--public` makes a client with no secret
(PKCE only), for desktop and mobile apps. `--redirect` can be repeated.

Redirect URIs must be `https`, `http` on a loopback address, or an app's custom scheme. Loopback URIs match any port
(RFC 8252); others must match exactly.

### In the logs

- `oauth-server: this collection may sign in but can't own API tokens; add it to tokens.Config.Collections`, once per
  collection, when Tokens is narrowed in a Go build. The page then says signing in with that account type "isn't set
  up on this server yet".
- `oauth-server: auth methods` (error) when a collection's sign-in methods can't be read.
- `oauth-server: cleanup` (warning) when the half-hourly removal of expired codes fails.

## Integrations

### Works with

| Plugin | How they fit |
|---|---|
| [Tokens](../tokens/README.md) | Required. Access tokens are API tokens owned by whoever signed in, with the client's id, the granted scopes and the audience. Every scope registered with Tokens can be requested. |
| [Remotes](../remotes/README.md) | OAuth Server lists itself as the `oauth` feature in `/_pocker/whoami`; `pocketone login` then signs in in the browser (loopback redirect with PKCE) or with a device code, and renews through the refresh token. |
| [MCP](../mcp/README.md) | MCP points clients here from its protected resource metadata, turns on self-registration, and makes tokens bound to `/mcp` never expire. Tool calls run as whoever signed in. |
| [Deploy](../deploy/README.md) | `pocketone login` gives the deploy commands a token for the account that signed in. |
| [Pages](../pages/README.md) | `pocketone oauth eject` writes the pages as PocketPages templates, rendered from then on. |

Under [Pocker](../pocker/README.md), a Go build registers OAuth Server (and Tokens) in each instance's `SetupApp`;
there's no pluginkit plugin for it yet. The stock `pocketone` binary doesn't add it to pocker instances.

### Audience (RFC 8707)

A `resource` parameter on `/oauth/authorize`, `/oauth/device` or `/oauth/token` (a URL on this server) binds the
access token to it: the token then only works under that URL's path. So a token issued for `/mcp` can't deploy, and a
deploy token can't drive MCP. Refreshed tokens keep it.

### Building on OAuth Server

- **Go API.** `oauthserver.For(app)` returns the server. `Issuer(e)` is its public base URL;
  `EnableDynamicRegistration()` turns on `POST /oauth/register`; `SetAccessTTL("/mcp", 0)` sets the lifetime of
  tokens bound to a resource (0: never expire). `FindClient`, `ListClients`, `AddClient` and `RemoveClient` manage
  clients.
- **Scopes.** Register a scope with Tokens (Go or `$tokens.registerScope`) and clients can ask for it; nothing else
  is needed.
- **Render the pages yourself.** `Config.Render(e, page, data)` renders any page (return `handled=false` to fall
  back), with `PageData`. It wins over ejected templates.

## Endpoints

| Path | What |
|---|---|
| `GET /.well-known/oauth-authorization-server` | Metadata (RFC 8414). The issuer is `Config.Issuer`, else the request's own scheme and host. |
| `GET/POST /oauth/authorize` | Sign-in and consent pages. Authorization code with PKCE (S256 only; required). |
| `POST /oauth/token` | `authorization_code`, `refresh_token` and `urn:ietf:params:oauth:grant-type:device_code`. Refresh rotates without logouts: the old access token lives out its time, and the old refresh token works again for 2 minutes, so two processes refreshing at once both succeed. |
| `POST /oauth/device`, `GET/POST /oauth/device/verify` | Device flow (RFC 8628): codes like `WDJB-MJHT`, valid 10 minutes. |
| `POST /oauth/revoke` | Revocation (RFC 7009): an access token or a refresh token (revokes both). |
| `GET/POST /oauth/provider/callback` | Where OAuth2 providers send the browser back after sign-in. |
| `POST /oauth/register` | Dynamic client registration (RFC 7591), when `DynamicRegistration` is on or a plugin called `EnableDynamicRegistration()` (MCP does). Public clients by default. |

Protected resource metadata (RFC 9728) belongs to the resource: MCP serves `/.well-known/oauth-protected-resource`.

## Clients

- **`pocketbase-cli`** is built in: public, any `http://127.0.0.1:<port>/…` or `[::1]` redirect, device codes. It's
  what `pocketone login` uses.
- **Client ID metadata documents (CIMD):** an `https://` client id is fetched as a JSON document (`client_id` must
  equal the URL; its `client_name` and `redirect_uris` are used; public clients only; cached 10 minutes). Only
  `Config.CIMDHosts` are trusted (default `claude.ai`, `claude.com`); the metadata says
  `client_id_metadata_document_supported`.
- Registered clients live in `_oauth_clients`; codes, device codes and refresh tokens in `_oauth_codes`.

## How sign-in works

The pages never check passwords or codes themselves. Each step calls PocketBase's own auth endpoint in-process,
through the app's router, so the collection's auth rule, MFA, rate limits, superuser IP limits and every
`onRecordAuth*Request` hook apply as for the REST API, and the record the endpoint returns is the one signed in:

- password: `POST /api/collections/{c}/auth-with-password`;
- emailed code: `POST …/request-otp`, then `POST …/auth-with-otp`;
- OAuth2: the browser goes to the provider with the state and PKCE from `GET …/auth-methods`, comes back to
  `/oauth/provider/callback`, and `POST …/auth-with-oauth2` completes it (account linking and sign-up are
  PocketBase's);
- MFA: when an auth call answers 401 with an `mfaId`, the second page passes it along.

Only collections with a sign-in method on are offered. Access tokens are owned by the signed-in record, so every
collection that may sign in must also be allowed by Tokens (its default is every auth collection). The session is a
15-minute HttpOnly cookie on `/oauth`; forms carry a CSRF token; a provider round trip keeps its state in a signed
10-minute cookie.

## Go config

```go
oauthserver.MustRegister(app, oauthserver.Config{
    Collections:         nil,                  // who may sign in; default: _superusers + every auth collection
    AccessTTL:           30 * 24 * time.Hour,  // default; per resource with srv.SetAccessTTL("/mcp", 0)
    RefreshTTL:          180 * 24 * time.Hour, // default, renewed on each refresh
    DynamicRegistration: false,                // POST /oauth/register (MCP turns it on)
    CIMDHosts:           nil,                  // default claude.ai, claude.com; an empty non-nil slice turns CIMD off
    Issuer:              "",                   // public base URL; default: the request's scheme and host
    Branding:            oauthserver.Branding{LogoURL: "/logo.svg", Accent: "#4f46e5", Footer: "Acme Inc."},
})
```

`Branding` also takes a `Name` (default: Settings → Application name). `Accent` is the button color (default
`#16161a`).

## Custom page templates

Ejected templates get `ctx.data`: `PageData` in camelCase (`data.appName`, `data.action`, `data.hidden`,
`data.csrf`, `data.clientName`, `data.scopes`, `data.superuser`, `data.providers`, `data.error`, …).
- Every POST form must include `include('/oauth/hidden')` and a `step` button.
- Steps: `password`, `request_otp`, `otp`, `oauth2:<provider>`, `use:<collection>` (switch forms; `use:` is the
  default user form), `allow`, `deny`, `switch`.
- Sign-in forms post `collection`; the MFA and code pages also post `mfa_id` / `otp_id`.

## Example

```bash
go run ./example serve --http=127.0.0.1:8090
go run ./example login http://127.0.0.1:8090    # opens the browser; --device for a code
```

[`example/main.go`](example/main.go) registers Tokens, Remotes and OAuth Server with self-registration on and some
branding. Logins from [devusers](../devusers/devusers.go): user@example.com / password on the default form,
admin@example.com / password after **Sign in as a superuser**.

## License

MIT
