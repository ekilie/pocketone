# tokens

API tokens for PocketBase: long-lived bearer secrets (`pbt_…`) owned by an auth record, optionally limited to scopes,
listed and revoked one by one. Send one as `Authorization: Bearer pbt_…` and the request runs as the token's owner.
JWTs keep working unchanged.

**Runs on:** Single app, or many under [pocker](../pocker/README.md).

```go
app := pocketbase.New()
tk := tokens.MustRegister(app, tokens.Config{
    Collections: []string{"_superusers", "users"}, // who may own tokens (default: every auth collection)
    MaxTTL:      0,                                // cap on a token's lifetime (0: none)
})
```

Try it: `go run ./example serve` (logins from [devusers](../devusers/devusers.go): admin@example.com / password).

## What it adds

- **`_tokens` collection:** owner (collection id + record id), name, `hash` (sha256 of the secret; the secret is
  never stored), display prefix, scopes, expiry, last use (time + IP), OAuth client id. Owners can list, view and
  delete their own tokens through the records API.
- **Bearer auth:** a middleware just before PocketBase's JWT middleware. Invalid or expired tokens leave the request
  unauthenticated, like an invalid JWT. A token can't be exchanged for a JWT (`auth-refresh` and `impersonate`
  refuse it), so revoking a token really ends its access.
- **Routes:** `GET /api/tokens` (yours; superusers see all, `?ownerCollection=&owner=` filters), `POST /api/tokens`
  (`{name, scopes, ttl | expires}`; superusers may add `ownerCollection` + `owner`), `DELETE /api/tokens/{id}`,
  `POST /api/tokens/revoke-all`, `GET /api/tokens/scopes`. The secret is returned once, on create.
- **Dashboard:** Settings → API tokens (create, see last use, revoke).
- **CLI** (on a `*pocketbase.PocketBase`): `token create --superuser admin@example.com --name ci --scope deploy
  [--ttl 720h]` prints the secret; `token list`, `token revoke <id>… | --all --superuser <email>`, `token scopes`.
- A daily cron deletes expired tokens; deleting an owner deletes its tokens.

## Scopes

A token without scopes has its owner's full rights. Scopes come from plugins and from the app itself, in Go or in
`pb_hooks`, and every registered scope appears with its description in Settings → API tokens and on the OAuth consent
page.

```go
tk.MustRegisterScope(tokens.Scope{
    Name:        "notes:read",                                 // lower-case letters, digits, ":" and "_"
    Description: "Read notes",                                 // shown in the token screen and on the consent page
    Paths:       []string{"/api/collections/notes/records"},  // URL prefixes where a token holding it authenticates
    Methods:     []string{"GET"},                              // optional: only these methods (GET covers HEAD)
})
se.Router.POST("/api/notes/archive", h).Bind(apis.RequireAuth(), tokens.RequireScope("notes:write"))
```

```js
// pb_hooks (the standard binary binds $tokens)
$tokens.registerScope({ name: "notes:write", description: "Change notes", paths: ["/api/collections/notes/records"] })
routerAdd("POST", "/api/notes/archive", (e) => { /* ... */ }, $apis.requireAuth(), $tokens.requireScope("notes:write"))
$tokens.hasScope(e, "notes:write")  // in a handler
$tokens.ruleField("notes:write")    // "@request.headers.x_token_scope_notes_write"
$tokens.scopes()                    // every registered scope, with its header and rule field
```

- **Where a scoped token works:** on the paths (and methods) of its scopes, plus `/_pocker/whoami`. Elsewhere: 403.
  A Go plugin can add a `Match func(*core.RequestEvent) bool` guard for requests that paths can't describe.
- **Guards:** `tokens.RequireScope("x")` (JS: `$tokens.requireScope("x")`) refuses an anonymous request and a token
  without the scope. `tokens.HasScope(e, "x")` is true for JWTs and unscoped tokens.
- **Collection rules:** every signed-in request carries `X-Token-Scope-<Name>: 1` for each scope it holds (a JWT or
  an unscoped token: every registered scope; a scoped token: only its own; anonymous: none). Headers a client sends
  with that prefix are always dropped. Rules read them as `@request.headers.x_token_scope_<name>`, with `:` as `_`:

  ```
  @request.auth.id = owner && @request.headers.x_token_scope_notes_write = "1"
  ```

  Names whose headers would collide (`notes:write` and `notes_write`) can't both be registered.
- Registering a name again merges its paths, methods and guards (two plugins sharing a scope, or hooks reloading).
- Plugins that don't import this package can read the request store keys `tokens.scopes` and `tokens.id`.

## Audience

A token created with `CreateOptions.Audience` (oauth-server sets it from the RFC 8707 `resource` parameter) only
authenticates under that URL's path, plus `/_pocker/whoami`; elsewhere it gets 403. `tokens.FromRequest(e).Audience`
tells a route what the token was issued for.

## Go API

`tokens.For(app)` returns the service: `Create(owner, CreateOptions{Name, Scopes, Expires, ClientId})` → secret +
record, `Verify(secret)`, `List(owner)`, `Revoke(id)`, `RevokeAll(owner)`, `DeleteExpired()`, `RegisterScope`,
`Scopes()`, `MustRegisterScope`, `BindJS(vm, app)` (the `$tokens` global; the plugin also implements
`pluginkit.JSBinder`). [oauth-server](../oauth-server/README.md) issues its access tokens through `Create`.

## License

MIT
