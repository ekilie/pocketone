# mcp

A [Model Context Protocol](https://modelcontextprotocol.io) endpoint for a PocketBase app: a thin mapping of the app's REST API for agents (a Claude connector, Claude Code, any Streamable HTTP client). No sidecar process.

- **Every tool is one REST call.** A call turns the tool's arguments into the request, runs it through the app's own router in-process as the caller, and turns the response back into an MCP result. API rules, request hooks, middleware and rate limits apply because it *is* the REST request. `ext/mcp` makes no permission decisions of its own.
- **The whole stock API is built in** as `pb_*` tools (records, files, auth, collections, settings, logs, crons, SQL, backups). An app that only uses the stock API needs nothing else.
- **Your own endpoints get tools the same way:** write the REST endpoint, then the tool wrapper (Go `AddEndpoint`, JS `$mcp.addEndpoint`).
- MCP Streamable HTTP, stateless: each POST carries one JSON-RPC message and gets JSON back. Protocol versions `2025-11-25`, `2025-06-18` and `2025-03-26`. Tools only for now.
- Sign-in with OAuth through `ext/oauth-server`, the way Claude connectors and Claude Code expect it, or a pasted PocketBase auth token.

```go
app := pocketbase.New()
server := mcp.MustRegister(app, mcp.Config{})

// the endpoint, as you'd write it anyway
app.OnServe().BindFunc(func(se *core.ServeEvent) error {
    se.Router.POST("/api/blog/posts/{id}/publish", publish).Bind(apis.RequireSuperuserAuth())
    return se.Next()
})

// the tool wrapper
server.MustAddEndpoint(mcp.Endpoint{
    Name:        "blog_publish_post",
    Description: "Publish a draft post and email subscribers.",
    Method:      http.MethodPost,
    Path:        "/api/blog/posts/{id}/publish",
    InputSchema: map[string]any{"type": "object", "required": []any{"id"},
        "properties": map[string]any{"id": map[string]any{"type": "string"}}},
})
```

A plugin registered on the same app reaches the server with `mcp.For(app)`.

## Sign-in

A tool call runs as whoever signed in to `/mcp`: a superuser gets what the API gives superusers, an app user what the collection rules give them.

**OAuth** (with `ext/tokens` and `ext/oauth-server` on the app; both are in the standard PocketOne binary):

1. A request without a token gets `401` with `WWW-Authenticate: Bearer resource_metadata="https://<host>/.well-known/oauth-protected-resource/mcp"`.
2. The protected resource metadata names the app as its authorization server. oauth-server publishes the authorization server metadata, with PKCE S256, CIMD and dynamic registration.
3. The client registers itself (Claude's published identity through CIMD, or dynamic registration, which `ext/mcp` turns on), and the user signs in in a browser.
4. The access token is an API token bound to `https://<host>/mcp` (RFC 8707). It carries the account's rights, like a PocketBase auth token, but works only on `/mcp`; tool calls reach the REST API with a PocketBase auth token for the same account, made per MCP request (as `ext/pages`' `ctx.pb()` does). A token bound to `/mcp` but limited to scopes is refused, since tool calls would run with full rights. MCP access tokens don't expire (`ext/mcp` sets that with oauth-server's `SetAccessTTL`), so an agent's connection isn't signed out; revoke one to end it.

```bash
claude mcp add --transport http my-app https://<host>/mcp
claude mcp login my-app        # or /mcp in a session
```

**Pasted tokens.** A PocketBase auth token (or an API token not bound to any resource) works on `/mcp` as it does on the REST API, and tool calls forward it unchanged.

Who may sign in through OAuth comes from `oauthserver.Config.Collections` (and `tokens.Config.Collections`). Without oauth-server, `/mcp` uses `PocketBaseTokenAuth` (pasted tokens; `Collections` limits which auth collections, default any). The authenticator is chosen on serve, so plugin registration order doesn't matter. `Config.Auth` overrides it.

## Tools

```go
mcp.Endpoint{
    Name:        "blog_publish_post",            // lowercase <plugin>_<verb>_<noun>; pb_ is the built-ins'
    Title:       "Publish post",
    Description: "What it does, for the model.",
    Method:      "POST",
    Path:        "/api/blog/posts/{id}/publish", // {id} is filled from args.id
    InputSchema: map[string]any{...},            // JSON Schema; default: no arguments
    Annotations: mcp.Write,                      // default from the method: GET read-only, DELETE destructive
    Request:     func(c *mcp.Call) (*mcp.Request, error) { ... }, // optional: shape the request
    Response:    func(c *mcp.Call, res *mcp.Response) (any, error) { ... }, // optional: shape the result
}
```

The default mapping:

- `{name}` path segments come from the argument of the same name, escaped. Empty, `.` and `..` are refused (the router would clean them into another route).
- Other arguments go to the query string for `GET`, `HEAD` and `DELETE`, and to the JSON body for other methods.
- An argument holding a file, `{"name": "a.png", "base64": "…", "type": "image/png"}` or a list of them, makes the body multipart (other arguments ride along as `@jsonPayload`), the way PocketBase takes uploads. Up to 10 MB per file.
- Arguments are checked against the schema's `required`, top-level `type`s, `enum`s and `additionalProperties: false` before the request is made; the endpoint validates the rest.
- The result: 2xx JSON comes back as the result (also as `structuredContent` when it's an object); other 2xx bodies as text; 4xx/5xx as a tool error with PocketBase's `{status, message, data}`, so the model can fix its call. JSON results are capped at 1 MB and say when they were cut.

`Request` sees the caller (`c.Auth()`) but can't change who the request runs as: an `Authorization` header it sets is dropped. Build paths with `c.Path(template)` or `mcp.PathValue`, never by joining raw strings. `c.Do` makes another request as the same caller (e.g. a file token before a download). `Response` may return any JSON-able value, a string, or an `mcp.Content` list (images, text, embedded files).

One tool serves every caller; the endpoint's rules decide what each gets. A wrapper can sit over a stock endpoint too, to give it the app's meaning (e.g. `notes_add_note` over `POST /api/collections/notes/records`, filling `owner` from `c.Auth()`).

Agents are told to prefer the app's own tools: when an app has any, the `initialize` instructions say to use them when one fits and fall back to `pb_*`, and `tools/list` lists them first. Put work that spans collections in a custom endpoint (one transaction, the app's checks), not in a chain of `pb_*` calls.

### Tools from a plugin

A PocketOne plugin that adds tools names them `<plugin>_<verb>_<noun>`: its name (with `-` as `_`), then at least two lowercase words, such as `notes_list_notes` or `oauth_server_revoke_client`. Each has a description, and an input schema, when it has one, is a JSON Schema object. The conformance test checks all of this when the plugin's test puts an MCP server on the test apps:

```go
conformance.Run(t, newPlugin, conformance.Options{Setup: func(app core.App) error {
    _, err := mcp.Register(app, mcp.Config{})
    return err
}})
```

## From JS (pb_hooks)

```go
jsvm.MustRegister(app, jsvm.Config{OnInit: func(vm *goja.Runtime) { mcp.BindJS(vm, app) }})
```

```js
// pb_hooks/blog.pb.js
routerAdd('POST', '/api/blog/posts/{id}/publish', (e) => { /* … */ }, $apis.requireSuperuserAuth())

$mcp.addEndpoint({
  name: 'blog_publish_post',
  description: 'Publish a draft post and email subscribers.',
  method: 'POST',
  path: '/api/blog/posts/{id}/publish',
  inputSchema: { type: 'object', required: ['id'], properties: { id: { type: 'string' } } },
  // optional; request is the default mapping, return a changed copy
  request: ({ args, auth, request }) => ({ ...request, body: { at: args.when } }),
  // optional; throw for an error the model can read
  response: ({ status, json }) => (status === 200 ? { published: json.id } : json),
})
$mcp.instructions('blog_publish_post also emails subscribers.')
```

`request` gets `{args, auth, request}` and returns `{method, path, query, body, headers, files}` (missing keys keep the default; `$mcp.pathValue(v)` escapes a path segment). `response` gets `{args, auth, status, headers, body, json, truncated}`. Both run one at a time on the VM that loaded the hooks and may use the file's top-level variables; the REST call itself runs outside it. Call `$mcp.addEndpoint` only at the top level of a hooks file.

## Built-in tools

One per stock endpoint, built with the same `Endpoint` API.

| Group | Tools | Who |
|---|---|---|
| `health` | `pb_check_health` | everyone |
| `records` | `pb_list_records` (`perPage` 20 by default), `pb_get_record`, `pb_create_record`, `pb_update_record`, `pb_delete_record`, `pb_run_batch` | per collection rules |
| `auth` | `pb_list_auth_methods`, `pb_request_otp`, `pb_request_verification`, `pb_confirm_verification`, `pb_request_password_reset`, `pb_confirm_password_reset`, `pb_request_email_change`, `pb_confirm_email_change` | per collection |
| `files` | `pb_download_file` (protected files through a file token made as the caller; images as images, text as text, others as embedded data; up to 10 MB) | per view rules |
| `collections` | `pb_list_collections`, `pb_get_collection`, `pb_create_collection`, `pb_update_collection`, `pb_delete_collection`, `pb_truncate_collection`, `pb_import_collections`, `pb_get_collection_scaffolds`, `pb_list_oauth2_providers`, `pb_dry_run_view` | superusers |
| `settings` | `pb_get_settings`, `pb_update_settings`, `pb_test_s3`, `pb_test_email`, `pb_generate_apple_client_secret` | superusers |
| `logs` | `pb_list_logs`, `pb_get_log`, `pb_get_log_stats`, `pb_truncate_logs` | superusers |
| `crons` | `pb_list_crons`, `pb_run_cron` | superusers |
| `sql` | `pb_run_sql` | superusers |
| `backups` | `pb_list_backups`, `pb_create_backup`, `pb_upload_backup`, `pb_download_backup`, `pb_delete_backup`, `pb_restore_backup` | superusers |

- Record create/update take field values in `data`, with files as above (`"field+"` appends, `"field-"` removes).
- `pb_delete_collection`, `pb_truncate_collection`, `pb_delete_backup` and `pb_restore_backup` need `confirm` set to the name.
- Superuser-only tools are left out of other callers' `tools/list` (presentation only; the endpoint still decides).
- Endpoints that hand out a new token (auth-with-*, auth-refresh, impersonate) and realtime (a stream) aren't tools.
- With `ext/s3redirect`, a download follows its one redirect to storage.
- Each tool call also counts against the API's rate limits, like any request. Batch sub-requests skip route middleware (PocketBase behaviour).

`Config.BuiltinGroups` picks groups, `ReadOnlyBuiltins` keeps only the read-only tools, and `NoBuiltins` drops them all.

## Config

| Field | Default | |
|---|---|---|
| `Path` | `/mcp` | endpoint path |
| `Name`, `Version`, `Description`, `Icons` | app name slug, `dev` | `serverInfo` (its title is Settings → Application name) |
| `Instructions` | generated | first paragraph of `instructions` |
| `PublicURL` | the request's host | origin used for the OAuth resource URL. The resource must match the URL clients use, so set it when a proxy rewrites the host. |
| `Auth` | `OAuth{}` with oauth-server, else `PocketBaseTokenAuth{}` | the `Authenticator` |
| `MaxResponse` | 1 MB | cap on a JSON result |
| `MaxFile` | 10 MB | cap on one file uploaded or downloaded through a tool |
| `AllowedOrigins` | none | extra browser origins. Requests with any other `Origin` header get 403. Clients that send no `Origin`, such as Claude's servers and Claude Code, pass. |

## Example

```bash
cd example
go build -o pbmcp .
./pbmcp superuser upsert test@example.com testpassword123 --dir=pb_data
./pbmcp serve --dir=pb_data
```

The example registers `ext/devusers`, so the logins are `admin@example.com` / `password` (superuser) and `user@example.com` / `password` (user). It also registers `ext/tokens` and `ext/oauth-server` for OAuth sign-in, and adds a tool over a Go route (`example_get_whoami`) and one over a JS route (`example_list_recent_users`, in `pb_hooks/mcp.pb.js`). Then: `claude mcp add --transport http pb-example http://127.0.0.1:8090/mcp` and `claude mcp login pb-example`.

## License

MIT
