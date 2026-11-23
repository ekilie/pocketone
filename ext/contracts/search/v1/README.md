---
owners: ["@benallfree"]
summary: "Named buckets of content an app searches by meaning: sources (files, folders, zips, pages, Git repositories, collections) kept current, one embedding model per app."
---

# search/v1

Named buckets of content an app searches by meaning. An app makes the bucket it needs (`ensure`); its admins fill it
with sources, each one tracked unit however many documents it brings: a file, pasted text, a folder, a zip, a web
page, a Git repository or one of the app's collections. Adding a source of the same name again replaces it (unchanged
documents cost nothing, missing ones go); pages and repositories are read again on an interval, collections kept in
sync. The provider splits each document into chunks that keep their source (document, section, lines, record), embeds
them with the app's one embedding model (llm.embed/v1, picked automatically; identical chunks anywhere in the app are
embedded once), and returns the best chunks for a query across the buckets asked for. It never answers the question
itself; llm.text/v1 and llm.chat/v1 calls take `enrich` to add a search's passages to their prompt, through the
[`Enrich`](enrich.go) helper every provider of those contracts uses.

A wire contract at `/api/_c/search/v1` ([search.go](search.go)), one provider. The [search](../../../search/README.md)
extension provides it. It brings the dashboard's **Search** screen ([ui/main.js](ui/main.js)), which calls only this
contract (and llm.embed/v1's model list), so it works whoever provides it.

## Use

In page code it's `$search` (bound by the search extension), and `enrich` on `$llm` calls:

```js
$search.ensure({ name: "help", title: "Help center", description: "Our help articles." })
$search.addURL({ bucket: "help", url: "https://github.com/acme/docs", every: "day" })
const r = await $search.searchAsync({ buckets: ["help"], query: "export to PDF", limit: 5, viewer: ctx.auth })
const answer = await $llm.text.completeAsync({ model: "vultr/glm-5.3", prompt: question, enrich: ["help"] })
```

Over HTTP (a superuser, or a token with the scope shown):

| Operation | HTTP | Body | Token scope |
|---|---|---|---|
| Status | `GET /api/_c/search/v1/status` | | `search:read` |
| Set model | `POST /api/_c/search/v1/set-model` | `{"model"}` (`""` picks one again) | `search` |
| Buckets | `GET /api/_c/search/v1/buckets` | | `search:read` |
| Bucket | `GET /api/_c/search/v1/bucket` | `?name=` | `search:read` |
| Ensure | `POST /api/_c/search/v1/ensure` | `{"name", "title", "description", "owner"}` | `search` |
| Update | `POST /api/_c/search/v1/update` | `{"name", "title"?, "description"?, "maxDocuments"?, "maxBytes"?}` | `search` |
| Delete | `POST /api/_c/search/v1/delete` | `{"name"}` | `search` |
| Sources | `GET /api/_c/search/v1/sources` | `?bucket=` | `search:read` |
| Add | `POST /api/_c/search/v1/add` | `{"bucket", "name", "text", "kind"?, "replace"?}` (a file or text) | `search` |
| Upload | `POST /api/_c/search/v1/upload` | `?bucket=&name=&replace=`, the file (or `.zip`) as the body | `search` |
| Put | `POST /api/_c/search/v1/put` | `{"bucket", "source", "path", "text", "replace"?}` (a folder's file) | `search` |
| Prune | `POST /api/_c/search/v1/prune` | `{"bucket", "source", "keep": [...]}` (ends a folder) | `search` |
| Add URL | `POST /api/_c/search/v1/add-url` | `{"bucket", "url", "kind"?, "every"?, "commits"?, "exclude"?}` | `search` |
| Detect URL | `POST /api/_c/search/v1/detect-url` | `{"url"}` → `{"kind"}`: `git` when the server answers the Git protocol, else `page` | `search:read` |
| Follow | `POST /api/_c/search/v1/follow` | `{"bucket", "collection", "fields": [...], "filter"?}` | `search` |
| Refresh | `POST /api/_c/search/v1/refresh` | `{"bucket", "source"}` | `search` |
| Remove | `POST /api/_c/search/v1/remove` | `{"bucket", "sources": [...]}` | `search` |
| Documents | `GET /api/_c/search/v1/documents` | `?bucket=&source=&prefix=` | `search:read` |
| Search | `POST /api/_c/search/v1/search` | `{"buckets": [...], "query", "limit", "prefix", "viewer"}` | `search:read` |

A source answers `{"bucket", "name", "kind", "url", "every", "commits", "exclude", "collection", "fields", "filter",
"hash", "status", "error", "documents", "chunks", "embedded", "bytes", "pending", "costMicroUSD", "refreshed", "next"}` (`embedded` below `chunks`: embedding under way; documents carry the same); `kind` is
`file`, `text`, `folder`, `zip`, `page`, `git` or `collection`, `status` is `reading`, `ready` or `error`, and `hash` is
a file's or zip's sha256, to spot an exact duplicate under another name. `replace` names an existing source the new one
replaces under its new name. Search answers `{"results": [{"bucket", "source", "document", "text", "heading",
"startLine", "endLine", "record", "score"}], "model", "usage"}`; no buckets searches every bucket. `viewer`
(`{"collection", "id"}`, or `{}` for a guest) leaves out records it can't view by the collection's view rule. Status
answers `{"model", "chunks", "embedded", "note"}`: `embedded` below `chunks` means embedding is under way.

Errors: `not_found` (no such bucket or source), `bad_request` (a bad name, URL, collection or field, an empty query,
binary content), `too_large` (over the bucket's limits, a document over 10 MB, a zip over 100 MB), `not_configured` (no
embedding model), and llm.embed/v1's errors from embedding the query.

From Go: `searchv1.Contract.Must(app)` (from `pocketone.dev/ext/contracts/search/v1`), and
`searchv1.Enrich(ctx, app, enrich, query)` for the passages a model call adds.

## Provide

Implement `searchv1.Search` and provide it in `Register` (`searchv1.Contract.Provide(app, impl)`). Keep these promises:

- `Ensure` is idempotent: an existing bucket keeps its settings; only an empty title, description or owner is filled in.
- A source is identified by its name within its bucket; adding one of the same name (or naming it in `replace`)
  replaces it, leaving sources in other buckets alone. Unchanged documents cost nothing; documents the new version
  doesn't have go.
- One embedding model for the app, so any buckets can be searched together; vectors from two models are never
  compared, a new model embeds every chunk again, and a chunk's content is embedded once however many sources hold it.
- A collection source's documents follow its records: saved, filtered out, deleted. Pages and repositories with an
  interval are read again when it comes round.
- With a `viewer`, no result comes from a record the viewer can't view.
- Results are chunks with their source, best first, at most `limit` (default 5, at most 50); `score` orders them.
- Searching is the only operation that waits on a model; adding and reading sources return before embedding is done.

## Extensions

<!-- contracts:index (generated by `go run ./.github/extensions contracts` from each extension's Contracts(); don't edit) -->

| Extension | |
|---|---|
| [search](../../../../ext/search/README.md) | provides |

<!-- /contracts:index -->

## License

MIT
