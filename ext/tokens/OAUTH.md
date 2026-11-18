# OAuth

[Tokens](README.md) includes an OAuth 2.1 authorization server whose access tokens are API tokens: the
[oauth/v1](../contracts/oauth/v1/oauth.go) contract. `pocketone login` signs in to it in the browser (or with a code on
another device), and MCP clients such as Claude or ChatGPT connectors register themselves and connect. Superusers and
app users sign in on your app's own pages, through PocketBase's own auth endpoints.

## Install

It comes with Tokens, which is in every app's [standard set](../standard/README.md). There's nothing to install or turn
on. Its endpoints are under `/api/_c/oauth/v1/` (see [Endpoints](#endpoints)); its metadata is at
`/.well-known/oauth-authorization-server`.

In your own Go build, configure it through Tokens:

```go
set := standard.New()
set.Tokens = tokens.New(tokens.Config{OAuth: tokens.OAuthConfig{
	Branding: tokens.Branding{Accent: "#4f46e5"},
}})
```

`tokens.Config{OAuth: tokens.OAuthConfig{Disabled: true}}` leaves it out.

## Configure

There is no `pocketone.yaml` section, flag or environment variable for OAuth Server. In `pocketone`:
- superusers and every auth collection may sign in;
- access tokens last 30 days, refresh tokens 180 days (renewed on each refresh);
- tokens for [MCP](../mcp/README.md) never expire, since MCP turns that on for `/mcp`;
- client self-registration (`POST /api/_c/oauth/v1/register`) is on, since MCP turns it on;
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
**Register `<your app's URL>/api/_c/oauth/v1/provider/callback` as a redirect URI with each provider.**

The pages show **Settings → Application → Application name**.

### Behind a proxy

The server names itself (the issuer) after the scheme and host the browser used, with `https` when the request is TLS
or `X-Forwarded-Proto` says so. If a proxy rewrites the host, set `Config.Issuer` in a Go build so the issuer and the
provider callback URL stay stable.

### Scopes

A client may ask for any scope registered with [Tokens](../tokens/README.md#scopes), including your own from
`$tokens.registerScope`. `offline_access` is accepted (refresh tokens are always issued).

The consent page lets the user decide: **Full access** (no scopes: the account's full rights) or **Only these**, with a
box for each scope. The scopes the client asked for start ticked; the others the account may hold are there to tick,
except ones that only reach contract APIs under `/api/_c/` (like `deploy`), which appear only when asked for. With no scopes
asked, **Full access** starts selected. The token gets exactly what was ticked, and the token response's `scope` says
which.

### Your own sign-in pages

The built-in pages (sign-in, second step, one-time code, consent, device code, done, error) follow the system's light
or dark mode and work on phones. They're the `oauth` set of [Templates](../templates/README.md); to change them, eject
it:

```bash
pocketone template eject oauth      # asks before overwriting files you changed; --force doesn't ask
```

It copies the built-in Go templates to `pb_templates/oauth/`: `signin.tpl`, `mfa.tpl`, `otp.tpl`, `consent.tpl`,
`device.tpl`, `done.tpl`, `error.tpl`, and the shared `head.tpl` (with the CSS), `foot.tpl` and `hidden.tpl`. From then
on the server renders your copy of each file, read on every request; delete a file to go back to the built-in one. To
restyle every page, edit only `head.tpl`. See [Custom page templates](#custom-page-templates) for the data they get.

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
   as a superuser, and the access: **Full access** or **Only these** scopes (see [Scopes](#scopes)). Click **Allow** or
   **Deny**, or **Not <email>? Use another account**.

With no user collection allowed, the superuser form is the only one. A browser stays signed in to these pages for 15
minutes. Failed attempts are limited to 10 per IP in 5 minutes.

On another device (over SSH, say), the client shows a code like `WDJB-MJHT`. Open
`https://<your app>/api/_c/oauth/v1/device/verify`, enter it under **Connect a device**, sign in and allow. Codes last 10 minutes.

### Log in from the command line

```bash
pocketone login prod
```

With OAuth Server on the remote, [Remotes](../remotes/README.md) opens your browser on the pages above, or shows a
device code when there's no browser (`--device` asks for one). The client is the built-in `pocketone-cli`, named
**pocketone CLI** on the consent page (the name follows the command, so another build shows its own).

### See and revoke what's connected

Every sign-in creates an API token named "<client> (OAuth)". They're listed under **Settings → API tokens** with an
`oauth: <client id>` label; click **Revoke** to disconnect a client. Revoking the access token also ends its refresh
token. A client can revoke its own tokens at `POST /api/_c/oauth/v1/revoke`, which `pocketone logout` does.

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

- `tokens: oauth: this collection may sign in but can't own API tokens; add it to tokens.Config.Collections`, once per
  collection, when Tokens is narrowed in a Go build. The page then says signing in with that account type "isn't set
  up on this server yet".
- `tokens: oauth: auth methods` (error) when a collection's sign-in methods can't be read.
- `tokens: oauth: cleanup` (warning) when the half-hourly removal of expired codes fails.

## Integrations

### Works with

| Extension | How they fit |
|---|---|
| [Tokens](README.md) | It's part of Tokens. Access tokens are API tokens owned by whoever signed in, with the client's id, the granted scopes and the audience. Every registered scope can be requested. |
| [Remotes](../remotes/README.md) | remote/v1's whoami lists `oauth/v1` (and the `oauth` feature, its metadata); `pocketone login` then signs in in the browser (loopback redirect with PKCE) or with a device code, and renews through the refresh token. |
| [MCP](../mcp/README.md) | MCP points clients here from its protected resource metadata, turns on self-registration, and makes tokens bound to `/mcp` never expire. Tool calls run as whoever signed in. |
| [Deploy](../deploy/README.md) | `pocketone login` gives the deploy commands a token for the account that signed in. |
| [Templates](../templates/README.md) | The pages are its `oauth` set: `pocketone template eject oauth` copies them to `pb_templates/oauth/` to edit. Without Templates, the built-in pages. |

On a [multisite](../multisite/README.md) host every instance that runs PocketOne has it, through the standard set.

### Audience (RFC 8707)

A `resource` parameter on `/api/_c/oauth/v1/authorize`, `/api/_c/oauth/v1/device` or `/api/_c/oauth/v1/token` (a URL on this server) binds the
access token to it: the token then only works under that URL's path. So a token issued for `/mcp` can't deploy, and a
deploy token can't drive MCP. Refreshed tokens keep it.

### Building on the OAuth server

- **Go API.** `tokens.OAuth(app)` returns the server. `Issuer(e)` is its public base URL;
  `EnableDynamicRegistration()` turns on `POST /api/_c/oauth/v1/register`; `SetAccessTTL("/mcp", 0)` sets the lifetime of
  tokens bound to a resource (0: never expire). `FindClient`, `ListClients`, `AddClient` and `RemoveClient` manage
  clients.
- **Scopes.** Register a scope with Tokens (Go or `$tokens.registerScope`) and clients can ask for it; nothing else
  is needed.
- **Render the pages yourself.** `Config.Render(e, page, data)` renders any page (return `handled=false` to fall
  back), with `PageData`. It wins over ejected templates.

## Example

The OAuth server is on wherever Tokens is (its [example](example/main.go) runs it). To brand it and let MCP connectors
register themselves:

```go
set := standard.New()
set.Tokens = tokens.New(tokens.Config{OAuth: tokens.OAuthConfig{
	DynamicRegistration: true, // MCP connectors register themselves (ext/mcp turns this on anyway)
	Branding:            tokens.Branding{Accent: "#4f46e5", Footer: "Example OAuth server"},
}})
app := set.NewApp(pocketbase.Config{}, jsvm.Config{})
```

## Endpoints

| Path | What |
|---|---|
| `GET /.well-known/oauth-authorization-server` | Metadata (RFC 8414). The issuer is `Config.Issuer`, else the request's own scheme and host. |
| `GET/POST /api/_c/oauth/v1/authorize` | Sign-in and consent pages. Authorization code with PKCE (S256 only; required). |
| `POST /api/_c/oauth/v1/token` | `authorization_code`, `refresh_token` and `urn:ietf:params:oauth:grant-type:device_code`. Refresh rotates without logouts: the old access token lives out its time, and the old refresh token works again for 2 minutes, so two processes refreshing at once both succeed. |
| `POST /api/_c/oauth/v1/device`, `GET/POST /api/_c/oauth/v1/device/verify` | Device flow (RFC 8628): codes like `WDJB-MJHT`, valid 10 minutes. |
| `POST /api/_c/oauth/v1/revoke` | Revocation (RFC 7009): an access token or a refresh token (revokes both). |
| `GET/POST /api/_c/oauth/v1/provider/callback` | Where OAuth2 providers send the browser back after sign-in. |
| `POST /api/_c/oauth/v1/register` | Dynamic client registration (RFC 7591), when `DynamicRegistration` is on or an extension called `EnableDynamicRegistration()` (MCP does). Public clients by default. |

Protected resource metadata (RFC 9728) belongs to the resource: MCP serves `/.well-known/oauth-protected-resource`.

## Clients

- **`pocketone-cli`** is built in: public, any `http://127.0.0.1:<port>/…` or `[::1]` redirect, device codes. It's
  what `pocketone login` uses.
- **Client ID metadata documents (CIMD):** an `https://` client id is fetched as a JSON document (`client_id` must
  equal the URL; its `client_name` and `redirect_uris` are used; public clients only; cached 10 minutes). Only
  `Config.CIMDHosts` are trusted (default `claude.ai`, `claude.com`); the metadata says
  `client_id_metadata_document_supported`.
- Registered clients live in `_tokens_oauth_clients`; codes, device codes and refresh tokens in `_tokens_oauth_codes`.

## How sign-in works

The pages never check passwords or codes themselves. Each step calls PocketBase's own auth endpoint in-process,
through the app's router, so the collection's auth rule, MFA, rate limits, superuser IP limits and every
`onRecordAuth*Request` hook apply as for the REST API, and the record the endpoint returns is the one signed in:

- password: `POST /api/collections/{c}/auth-with-password`;
- emailed code: `POST …/request-otp`, then `POST …/auth-with-otp`;
- OAuth2: the browser goes to the provider with the state and PKCE from `GET …/auth-methods`, comes back to
  `/api/_c/oauth/v1/provider/callback`, and `POST …/auth-with-oauth2` completes it (account linking and sign-up are
  PocketBase's);
- MFA: when an auth call answers 401 with an `mfaId`, the second page passes it along.

Only collections with a sign-in method on are offered. Access tokens are owned by the signed-in record, so every
collection that may sign in must also be allowed by Tokens (its default is every auth collection). The session is a
15-minute HttpOnly cookie on `/api/_c/oauth/v1`; forms carry a CSRF token; a provider round trip keeps its state in a signed
10-minute cookie.

## Go config

```go
tokens.New(tokens.Config{OAuth: tokens.OAuthConfig{
    Collections:         nil,                  // who may sign in; default: _superusers + every auth collection
    AccessTTL:           30 * 24 * time.Hour,  // default; per resource with srv.SetAccessTTL("/mcp", 0)
    RefreshTTL:          180 * 24 * time.Hour, // default, renewed on each refresh
    DynamicRegistration: false,                // POST /api/_c/oauth/v1/register (MCP turns it on)
    CIMDHosts:           nil,                  // default claude.ai, claude.com; an empty non-nil slice turns CIMD off
    Issuer:              "",                   // public base URL; default: the request's scheme and host
    Branding:            tokens.Branding{LogoURL: "/logo.svg", Accent: "#4f46e5", Footer: "Acme Inc."},
}})
```

`Branding` also takes a `Name` (default: Settings → Application name). `Accent` is the button color (default
`#16161a`).

## Custom page templates

The templates are Go `html/template` files, and their dot is `PageData` (`{{.AppName}}`, `{{.Action}}`, `{{.Hidden}}`,
`{{.CSRF}}`, `{{.ClientName}}`, `{{.Scopes}}`, `{{.ScopeChoices}}`, `{{.Superuser}}`, `{{.Providers}}`, `{{.Error}}`, …;
see `PageData` in [oauth_pages.go](oauth_pages.go)). Each page includes the others by name (`{{template "head" .}}`).
- Every POST form must include `{{template "hidden" .}}` and a `step` button.
- Steps: `password`, `request_otp`, `otp`, `oauth2:<provider>`, `use:<collection>` (switch forms; `use:` is the
  default user form), `allow`, `deny`, `switch`.
- Sign-in forms post `collection`; the MFA and code pages also post `mfa_id` / `otp_id`.
- Consent posts `access` = `full`, or `limited` with a `grant` field per ticked scope (from `data.scopeChoices`:
  `name`, `description`, `checked`). A consent form without `access` grants what the client asked for.

## License

MIT
