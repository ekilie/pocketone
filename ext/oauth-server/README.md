# oauth-server

Makes a PocketBase app an OAuth 2.1 authorization server whose access tokens are [API tokens](../tokens/README.md).
`pocketbase login` signs in to it in the browser (or with a code on another device), and MCP clients such as Claude
or ChatGPT connectors can register themselves and connect.

**Runs on:** Single app. Requires [tokens](../tokens/README.md).

```go
// tokens are owned by whoever signs in, so list every collection that may sign in (tokens' default is _superusers only)
tokens.MustRegister(app, tokens.Config{Collections: []string{"_superusers", "users"}})
remotes.MustRegister(app, remotes.Config{}) // optional: advertises "oauth" in /_pocker/whoami so login finds it
oauthserver.MustRegister(app, oauthserver.Config{
    Collections:         nil,                     // who may sign in; default: _superusers + every auth collection
    AccessTTL:           30 * 24 * time.Hour,     // default
    RefreshTTL:          180 * 24 * time.Hour,    // default, renewed on each refresh
    DynamicRegistration: false,                   // POST /oauth/register for MCP clients
    Branding:            oauthserver.Branding{LogoURL: "/logo.svg", Accent: "#4f46e5", Footer: "Acme Inc."},
})
```

Try it: `go run ./example serve`, then `go run ./example login http://127.0.0.1:8090` (logins from
[devusers](../devusers/devusers.go): user@example.com / password on the default form, admin@example.com / password
after "Sign in as a superuser").

## Endpoints

| Path | What |
|---|---|
| `GET /.well-known/oauth-authorization-server` | Metadata (RFC 8414). The issuer is `Config.Issuer`, else the request's own URL. |
| `GET/POST /oauth/authorize` | Sign-in and consent pages. Authorization code with PKCE (S256 only; required). |
| `POST /oauth/token` | `authorization_code`, `refresh_token` (rotated; the old access token is revoked), `urn:ietf:params:oauth:grant-type:device_code`. |
| `POST /oauth/device` + `/oauth/device/verify` | Device flow (RFC 8628): codes like `WDJB-MJHT`, valid 10 minutes. |
| `POST /oauth/revoke` | Revocation (RFC 7009): an access token or a refresh token (revokes both). |
| `GET/POST /oauth/provider/callback` | Where OAuth2 providers (Google, GitHub, …) send the browser back after sign-in. |
| `POST /oauth/register` | Dynamic client registration (RFC 7591), when `DynamicRegistration` is on or a plugin called `EnableDynamicRegistration()` (ext/mcp does). Public clients by default. |

Protected resource metadata (RFC 9728) belongs to the resource: ext/mcp serves `/.well-known/oauth-protected-resource`.

**Audience (RFC 8707).** A `resource` parameter on `/oauth/authorize`, `/oauth/device` or `/oauth/token` (a URL on this
server) binds the access token to it: the token then only authenticates under that URL's path, so a token issued for
`/mcp` can't deploy and a deploy token can't drive MCP. Refreshed tokens keep it. `offline_access` is accepted in
`scope` (refresh tokens are always issued) and listed in `scopes_supported`.

Access tokens are `pbt_…` API tokens with the client's id and the granted scopes, so they show up (and can be
revoked) under Settings → API tokens, and every scope a plugin registers can be requested. No scope requested means
the account's full rights; the consent page says so.

## Sign-in

**Who.** `Config.Collections` empty (the default) means `_superusers` plus every auth collection, resolved on each
request; a list narrows it. MCP tools and API calls then run as whoever signed in, under the REST API's rules. The
sign-in page signs in **app users** by default: the allowed auth collections other than `_superusers` that have a
sign-in method on (`users` first; with several, a small choice at the top). At the bottom, **Sign in as a superuser**
shows the same page against `_superusers`, styled clearly differently (its own background and accent, a "Superuser
sign-in" heading and a warning), with a link back. Without `_superusers` allowed there's no link; with no user
collection allowed, the superuser form is the only one. The consent page names the account and whether the client
will act as a user (and which collection) or as a superuser.

**Tokens.** Access tokens are owned by the signed-in record, so every collection that may sign in must also be in
`tokens.Config.Collections` (whose default is `_superusers` only). A collection that can't own tokens shows "isn't set
up on this server" on its form, refuses sign-in, and logs a warning naming it.

**How.** The pages never check passwords or codes themselves: each step calls PocketBase's own auth endpoint
in-process, through the app's router, so the collection's auth rule (e.g. `verified = true`), MFA, rate limits,
superuser IP limits and every `onRecordAuth*Request` hook apply as for the REST API, and the record the endpoint
returns is the one signed in. Only the methods the collection has on are offered (`GET …/auth-methods`):

- password: `POST /api/collections/{c}/auth-with-password`;
- emailed code: `POST …/request-otp`, then `POST …/auth-with-otp`;
- OAuth2: one button per provider on the collection (never for `_superusers`, which PocketBase doesn't allow). The
  browser goes to the provider with the state and PKCE from auth-methods, comes back to `/oauth/provider/callback`,
  and `POST …/auth-with-oauth2` completes it (account linking and sign-up are PocketBase's). **Register
  `<issuer>/oauth/provider/callback` as a redirect URI with each provider** (the issuer is `Config.Issuer`, else the
  scheme and host the browser used; set `Issuer` behind a proxy so it's stable);
- MFA: when an auth call answers 401 with an `mfaId`, a second page offers the collection's other methods and passes
  the `mfaId` along (after a password, when an emailed code is the only option left, it's sent right away).

Failed attempts are also limited per IP here (10 per 5 minutes). The session is a 15-minute HttpOnly cookie on
`/oauth`; forms carry a CSRF token; a provider round trip keeps its state in a signed 10-minute cookie.

## Clients

- **`pocketbase-cli`** is built in: public, any `http://127.0.0.1:<port>/…` or `[::1]` redirect, device codes. It's
  what `pocketbase login` uses ([remotes](../remotes/README.md)).
- **Client ID metadata documents (CIMD):** an `https://` client id is fetched as a JSON document (`client_id` must
  equal the URL; its `client_name` and `redirect_uris` are used; cached 10 minutes). Only `Config.CIMDHosts` are
  trusted (default `claude.ai`, `claude.com`, so Claude and Claude Code connect without registering); the metadata
  says `client_id_metadata_document_supported`.
- Registered clients live in `_oauth_clients`. Loopback redirect URIs match any port (RFC 8252); other URIs match
  exactly; plain `http` only for loopback.
- CLI: `pocketbase oauth clients list`, `oauth clients add --name "My app" --redirect https://…/callback [--public]`
  (prints the client id, and the secret once for a confidential client), `oauth clients rm <client_id>`.

## Branding and custom pages

The built-in pages (sign-in, one-time code, consent, device code, done, error) use Settings → Application name, plus
`Branding` (logo, accent color, footer). They follow the system's light or dark mode and work on phones.

To replace them:
- **Eject (with [pages](../pages/README.md)):** `pocketbase oauth eject` writes the pages as PocketPages templates to
  `pb_pages/_private/oauth/` (`signin.ejs`, `mfa.ejs`, `otp.ejs`, `consent.ejs`, `device.ejs`, `done.ejs`, `error.ejs`, and the
  shared `head.ejs` / `foot.ejs` / `hidden.ejs` with the CSS). It prints each file, refuses to overwrite existing ones
  unless `--force`, and from then on the server renders those templates (missing ones fall back to the built-in
  page; delete them to go back). `_private` keeps them from being served as routes. The data is `ctx.data`
  (`data.appName`, `data.action`, `data.hidden`, `data.csrf`, `data.clientName`, `data.scopes`, `data.superuser`,
  `data.providers`, `data.error`, …, `PageData` in camelCase); every POST form must include `include('/oauth/hidden')`
  and a `step` button. Steps: `password`, `request_otp`, `otp`, `oauth2:<provider>`, `use:<collection>` (switch
  forms; `use:` is the default user form), `allow`, `deny`, `switch`. Sign-in forms post `collection`; the MFA and
  code pages also post `mfa_id` / `otp_id`.
- **`Config.Render(e, page, data)`** renders any page itself (return `handled=false` to fall back), with
  `PageData`. It wins over ejected templates.

## License

MIT
