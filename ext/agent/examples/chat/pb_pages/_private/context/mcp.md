Source: https://pocketone.dev/docs/features/mcp

# MCP for AI agents

> **PocketBase:** no MCP server. **PocketOne:** every app serves MCP at `/mcp`, with OAuth sign-in, so Claude and other agents work with your app as you.

Point Claude, or any other MCP client, at your app and sign in. The agent can then read and change your data, check on
the app and run admin tasks, as you and with your rights, nothing more.

Every PocketOne app serves MCP at `/mcp`. It's on by default. You don't need to set anything up.

## Connect Claude

1. In your MCP client, add `https://<your app>/mcp` as a remote MCP server. In Claude, that's a custom connector.
2. A browser opens on your app's sign-in page. Sign in as an app user, or click **Sign in as a superuser**.
3. The consent page names the client and the account it will act as. Choose **Full access**, or **Only these** and tick
   what the agent may do.
4. Click **Allow**.

The agent stays connected until you disconnect it.

In Claude Code:

```bash
claude mcp add --transport http my-app https://<your app>/mcp
claude mcp login my-app        # or /mcp in a session
```

Claude and Claude Code connect without any setup on your side. Other MCP clients register themselves with your app
first.

## What the agent can do

Every tool call is one request to your app's own API, made as whoever signed in. Your API rules, hooks and rate limits
apply as they would to any request.

- **PocketBase's whole API**, as `pb_*` tools: records (`pb_list_records`, `pb_create_record`, …), files and auth for
  everyone; collections, settings, logs, crons, SQL and backups for superusers. SQL (`pb_run_sql`) goes straight to
  the database, past API rules and hooks.
- **Admin tasks** each feature adds, such as `restore_points` and `restore_restore_to` to
  [undo a mistake](https://pocketone.dev/docs/features/restore), or `remote_status` and `remote_logs` to check on the app. The
  [MCP reference](https://pocketone.dev/docs/reference/mcp) lists them all.
- **Your app's own tools**, from `pb_hooks` (below).

A superuser gets what the API gives superusers. An app user gets what your collection rules give them, and doesn't see
the superuser-only tools.

## Add your own tools

Write the endpoint as you would anyway, then declare its tool at the top level of a `pb_hooks` file:

```js
// pb_hooks/blog.pb.js
routerAdd('POST', '/api/blog/posts/{id}/publish', (e) => { /* … */ }, $apis.requireSuperuserAuth())

$mcp.addEndpoint({
  name: 'blog_publish_post',
  description: 'Publish a draft post and email subscribers.',
  method: 'POST',
  path: '/api/blog/posts/{id}/publish', // {id} comes from the tool's id argument
  inputSchema: { type: 'object', required: ['id'], properties: { id: { type: 'string' } } },
})
$mcp.instructions('blog_publish_post also emails subscribers.')
```

The tool runs the request as the caller, so the route's own auth check is what protects it. When your app has its own
tools, agents are told to prefer them over `pb_*`.

## What's covered

- **Limited to scopes:** with **Only these**, each call is checked like a request from an
  [API token](https://pocketone.dev/docs/features/tokens) with those scopes. Calls they don't cover get `403`, and tools they can't reach
  are hidden.
- **Only for MCP:** the agent's token works on `/mcp` and nowhere else.
- **Not tools:** sign-in endpoints that hand out a new token, and realtime subscriptions. A file download through
  `pb_download_file` is capped at 10 MB.
- **Browsers:** a web page on another site can't call `/mcp`. Clients that send no `Origin` header, such as Claude's
  servers and Claude Code, can.

Anyone who can sign in to your app can connect an agent, with their own rights. To give an agent less, choose
**Only these**, or sign it in as an account with fewer rights.

## Disconnect an agent

Each connection is an [API token](https://pocketone.dev/docs/features/tokens) named `<client> (OAuth)`. Open **Settings → API tokens**,
find the one with the `oauth: <client id>` label, and click **Revoke**.

## Paste a token instead

A client that can't sign in can send a PocketBase auth token, or an API token, as `Authorization: Bearer …`. An API
token with scopes works only if one of them covers `/mcp`, so to limit an agent, use the consent page instead.

## Change it or turn it off

There's no `pocketone.yaml` key for MCP: it always serves `/mcp`, and an agent can do nothing until someone signs it
in. The name agents see is your app's **Settings → Application → Application name**. Every key is in the
[pocketone.yaml reference](https://pocketone.dev/docs/config-reference).

`$mcp` isn't available in `pb_hooks` when the app runs PocketBase's own JS engine (`jsvm: goja`).

## How it works

- [MCP](https://pocketone.dev/docs/extensions/mcp) serves `/mcp` and turns each tool call into a request to your app's API.
- The [OAuth server](https://pocketone.dev/docs/extensions/api-tokens/oauth) in [API Tokens](https://pocketone.dev/docs/extensions/api-tokens) runs the sign-in and consent
  pages, and issues the agent's token.
- Admin tools come from the [contracts](https://pocketone.dev/docs/contracts) your app answers. Another MCP server can stand in through
  the [mcp/v1](https://pocketone.dev/docs/contracts/mcp/v1) contract and serve the same tools.

Looking to teach a coding agent how to build with PocketOne? That's a different thing: see
[Teach your coding agent](https://pocketone.dev/docs/agents).
