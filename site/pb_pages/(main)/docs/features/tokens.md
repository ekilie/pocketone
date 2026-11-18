---
title: API tokens and OAuth
description: Give a script, a CI job or a service its own long-lived token, limited to what it needs, and revoke it whenever you like. Apps and tools can also sign in to get one.
---

# API tokens and OAuth

> **PocketBase:** an auth token acts as its account, and changing the account's token key signs out all of its tokens at once. **PocketOne:** named, scoped tokens you revoke individually, and your app is an OAuth server that apps and agents sign in through.

A script, a CI job or another service can't type a password. Give it an API token instead: a long secret that starts
with `pbt_`, acts as the account that owns it, and can be limited to a few scopes and given an expiry. Revoke it, and
it stops working at once.

This is on for every PocketOne app. You don't need to set anything up. Ordinary PocketBase sign-ins keep working as
before.

## Create a token in the dashboard

1. Sign in to the dashboard as a superuser and open **Settings → API tokens** (under **Access**).
2. Fill in a **Name**, so you can recognize the token later.
3. Optionally set **Expires in (days)**. Leave it empty and the token never expires.
4. Tick the **Scopes** the token needs. Tick none and it gets its owner's full rights.
5. The token is yours by default. To make one for another account, fill in **Owner collection** and
   **Owner (id or email)**.
6. Click **Create token**, then copy it. It's shown only once and never stored.

Send it with each request:

```bash
curl -H "Authorization: Bearer pbt_…" https://myapp.example.com/api/collections/notes/records
```

## See and revoke tokens

The same screen lists every token: its name, its first characters, owner, scopes, expiry and when it was last used
(time and IP). Click **Revoke** and confirm to delete one. Anything using it stops working at once.

## Let apps and tools sign in

Some clients sign in for you instead of asking for a pasted token: `pocketone login`, Claude and other
[AI agents](/docs/features/mcp), or an app of your own. They send you to your app's own sign-in pages:

1. Sign in with whatever the collection allows: a password, an emailed code, or (for app users) Google and
   other providers, plus a second step if the account has MFA. Click **Sign in as a superuser** to sign in as one.
2. The consent page names the client and the account it will act as. Choose **Full access**, or **Only these** and
   tick the scopes it may use.
3. Click **Allow**.

On a machine with no browser (over SSH, say), the client shows a code instead. Open
`https://<your app>/api/_c/oauth/v1/device/verify`, enter it under **Connect a device**, sign in and allow. Codes last
10 minutes.

Each sign-in becomes an API token named `<client> (OAuth)`, bound to exactly what you approved. It shows up under
**Settings → API tokens** with an `oauth: <client id>` label. Click **Revoke** to disconnect the client.
`pocketone logout` revokes a browser login from your terminal.

## From your terminal

On the machine that runs the app, the `token` command works on the app's data folder, even before the first serve:

```bash
pocketone token create --superuser admin@example.com --name ci --scope deploy --ttl 720h   # prints the secret
pocketone token create --collection users --record user@example.com --name script
pocketone token list
pocketone token revoke <id>
pocketone token revoke --all --superuser admin@example.com
```

`--scope` can be repeated. Without `--ttl` the token never expires.

For CI that runs `pocketone` against your app, put the token in `P1_TOKEN` instead of signing in
([Remote CLI](/docs/features/remote)).

To let an app of your own sign people in, register it once:

```bash
pocketone oauth clients add --name "My app" --redirect https://myapp.example.com/callback
```

It prints the client id, and a secret once. Add `--public` for a desktop or mobile app with no secret. Claude, the
`pocketone` CLI and other MCP clients don't need this.

## From pb_hooks

Add your own scopes, and they show up in the dashboard and on the consent page:

```js
// pb_hooks/scopes.pb.js
$tokens.registerScope({ name: 'notes:read', description: 'Read notes',
  paths: ['/api/collections/notes/records'], methods: ['GET'] })

routerAdd('GET', '/api/notes/summary', (e) => { /* ... */ },
  $apis.requireAuth(), $tokens.requireScope('notes:read'))
```

A collection rule can check a scope too: `@request.headers.x_token_scope_notes_read = "1"`.

## What a token can do

- **No scopes:** everything its owner can do.
- **With scopes:** only the requests its scopes cover. Anything else gets `403`.
- **Which scopes:** the ones features add (`deploy`, and one for each admin feature, often with a `:read`
  version, such as `restore` and `restore:read`), plus your own. Some, like `deploy`, only superusers can hold. The
  [CLI reference](/docs/reference/cli) and [MCP reference](/docs/reference/mcp) name the scope each operation needs.
- **Expiry:** never, unless you set one. Tokens from a sign-in last 30 days, and the client renews them; AI agents'
  tokens never expire. Expired tokens are deleted once a day.
- **Gone with the account:** deleting an account deletes its tokens.
- **No way around a revoke:** a token can't be traded for a PocketBase auth token, so revoking it really ends its
  access.

Only superusers see **Settings → API tokens**. Other signed-in users manage their own tokens through the
[token API](/docs/contracts/tokens/v1). A token with scopes can only create tokens within those scopes.

## Change it or turn it off

There's no `pocketone.yaml` key for API tokens: they're always on, and a token does nothing until someone creates one.
To limit what a token can do, give it scopes and an expiry, or revoke it. Every key is in the
[pocketone.yaml reference](/docs/config-reference).

To restyle the sign-in pages, copy them into your app with `pocketone template eject oauth`.

## How it works

- [Tokens](/docs/extensions/tokens) stores the tokens (only a hash of each secret), checks them on every request and
  adds the dashboard screen.
- [OAuth server](/docs/extensions/tokens/oauth) runs the sign-in and consent pages and issues tokens for what you
  approved.
- They answer the [tokens/v1](/docs/contracts/tokens/v1) and [oauth/v1](/docs/contracts/oauth/v1) contracts.
- [Templates](/docs/extensions/templates) lets you restyle the sign-in pages.
