---
owners: ["@benallfree"]
summary: "Named buckets of content an app searches by meaning: files, text or a collection's records, one embedding model per app."
---

# search/v1

Named buckets of content an app searches by meaning. An app makes the bucket it needs (`ensure`); its admins load
documents into it (files or pasted text, by name), or the bucket follows one of the app's collections and keeps a
document per record in sync. The provider splits each document into chunks that keep their source (document, section,
lines, record), embeds them with the app's one embedding model (llm.embed/v1, picked automatically), and returns the
best chunks for a query across the buckets asked for. It never answers the question itself; llm.text/v1 and
llm.chat/v1 calls take `enrich` to add a search's passages to their prompt, through the [`Enrich`](enrich.go) helper
every provider of those contracts uses.

A wire contract at `/api/_c/search/v1` ([search.go](search.go)), one provider. The [search](../../../search/README.md)
extension provides it. It brings the dashboard's **Search** screen ([ui/main.js](ui/main.js)), which calls only this
contract (and llm.embed/v1's model list), so it works whoever provides it.

## Use

In page code it's `$search` (bound by the search extension), and `enrich` on `$llm` calls:

```js
$search.ensure({ name: "storytime", title: "Story Time library", description: "Stories to draw on." })
const r = await $search.searchAsync({ buckets: ["storytime"], query: "a fox by the sea", limit: 5, viewer: ctx.auth })
const answer = await $llm.text.completeAsync({ model: "vultr/glm-5.3", prompt: question, enrich: ["storytime"] })
```

Over HTTP (a superuser, or a token with the scope shown):

| Operation | HTTP | Body | Token scope |
|---|---|---|---|
| Status | `GET /api/_c/search/v1/status` | | `search:read` |
| Set model | `POST /api/_c/search/v1/set-model` | `{"model"}` (`""` picks one again) | `search` |
| Buckets | `GET /api/_c/search/v1/buckets` | | `search:read` |
| Bucket | `GET /api/_c/search/v1/bucket` | `?name=` | `search:read` |
| Ensure | `POST /api/_c/search/v1/ensure` | `{"name", "title", "description", "owner", "source"}` | `search` |
| Update | `POST /api/_c/search/v1/update` | `{"name", "title"?, "description"?, "source"?, "maxDocuments"?, "maxBytes"?}` | `search` |
| Delete | `POST /api/_c/search/v1/delete` | `{"name"}` | `search` |
| Add | `POST /api/_c/search/v1/add` | `{"bucket", "name", "text"}` | `search` |
| Upload | `POST /api/_c/search/v1/upload` | `?bucket=&name=`, the file as the body | `search` |
| Documents | `GET /api/_c/search/v1/documents` | `?bucket=&prefix=` | `search:read` |
| Remove | `POST /api/_c/search/v1/remove` | `{"bucket", "names": [...]}` | `search` |
| Reindex | `POST /api/_c/search/v1/reindex` | `{"bucket", "names": [...]}` (none: all) | `search` |
| Search | `POST /api/_c/search/v1/search` | `{"buckets": [...], "query", "limit", "prefix", "viewer"}` | `search:read` |

A `source` is `{"collection", "fields": [...], "filter"}`: each record the filter keeps is a document named
`<collection>/<id>`, the first field its title. Search answers `{"results": [{"bucket", "document", "text", "heading",
"startLine", "endLine", "record", "score"}], "model", "usage"}`; no buckets searches every bucket. `viewer`
(`{"collection", "id"}`, or `{}` for a guest) leaves out records it can't view by the collection's view rule. Status
answers `{"model", "chunks", "embedded", "note"}`; `embedded` below `chunks` means embedding is under way (after a
model change, or new content), and `note` says what's missing when there's no embedding model. Add returns the
document with `status` `queued` (waiting to be embedded), `embedding`, `ready` or `error`, and `unchanged` when the
same text was already there.

Errors: `not_found` (no such bucket or document), `bad_request` (a bad name or source, an empty query, binary content),
`too_large` (over the bucket's limits, or a document over 10 MB), `not_configured` (no embedding model), and
llm.embed/v1's errors from embedding the query.

From Go: `searchv1.Contract.Must(app)` (from `pocketone.dev/ext/contracts/search/v1`), and
`searchv1.Enrich(ctx, app, enrich, query)` for the passages a model call adds.

## Provide

Implement `searchv1.Search` and provide it in `Register` (`searchv1.Contract.Provide(app, impl)`). Keep these promises:

- `Ensure` is idempotent: an existing bucket keeps its settings; only an empty title, description, owner or source is
  filled in.
- `Add` with the same name replaces the document; the same text again changes nothing and says `unchanged`.
- One embedding model for the app, so any buckets can be searched together; vectors from two models are never
  compared, and a new model embeds every chunk again.
- A followed collection's documents follow its records: saved, filtered out, deleted.
- With a `viewer`, no result comes from a record the viewer can't view.
- Results are chunks with their source, best first, at most `limit` (default 5, at most 50); `score` orders them.
- Searching is the only operation that may call a model; adding returns before embedding is done.

## Extensions

<!-- contracts:index (generated by `go run ./.github/extensions contracts` from each extension's Contracts(); don't edit) -->

| Extension | |
|---|---|
| [search](../../../../ext/search/README.md) | provides |

<!-- /contracts:index -->

## License

MIT
