---
owners: ["@benallfree"]
version: 0.1.0
title: "Search"
group: binary
summary: "Buckets of content your app searches by meaning: files, text or a collection's records; $search, and enrich on $llm calls to answer from them."
---

# Search

Search keeps **buckets** of content your app searches by meaning. Your app makes the bucket it needs; you load it from
the dashboard's **Search** screen by dropping files or pasting text, or the bucket **follows a collection** and keeps
its records in sync. Each document is split into chunks that remember where they came from (the document, its section,
its lines, its record) and embedded with the app's one embedding model, picked for you from your providers in
Settings → LLM.

The usual use is answering from your own content. Name the buckets on a model call and it adds the best passages to
the prompt:

```ts
const answer = await $llm.text.completeAsync({ model: "vultr/glm-5.3", prompt: question, enrich: ["help"] })
// answer.passages: what was added, to show where the answer came from
```

Or search yourself:

```ts
$search.ensure({ name: "storytime", title: "Story Time library", description: "Stories, characters and places to draw on." })
const found = await $search.searchAsync({ buckets: ["storytime"], query: "a fox who lives by the sea", limit: 5 })
for (const r of found.results) console.log(r.bucket, r.document, r.heading, r.text)
```

In Go:

```go
s := searchv1.Contract.Must(app) // search/v1, from pocketone.dev/ext/contracts/search/v1
s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "storytime", Title: "Story Time library"})
res, err := s.Search(ctx, extkit.Internal, searchv1.SearchInput{Buckets: []string{"storytime"}, Query: "a fox who lives by the sea"})
```

## Install

Search is built into `pocketone` and on by default, in every app and every instance of a host (a standard member). In
your own Go build, next to LLM:

```go
extkit.Setup(app, []extkit.Extension{llm.New(), search.New()}, nil, nil)
```

The module is `pocketone.dev/ext/search`. Its collections (`_search_buckets`, `_search_documents`, `_search_chunks`)
and its keyword index (the SQLite FTS5 table `_search_fts`) come from its own migration, `0001_buckets` in
[schema.go](schema.go), which runs on the first start. They stay empty until something makes a bucket.

## Configure

Searching needs an AI provider with embedding models in Settings → LLM (Vultr's `qwen3-embedding-4b`, say). There's
nothing else to set:

- **The embedding model** is one for the whole app, so every bucket can be searched together. The first time anything
  needs it, Search picks the cheapest embedding model your providers list and keeps it (in kv, `_search/model`).
  Change it on the Search screen: every bucket is embedded again in the background, with its progress shown there, and
  search keeps working meanwhile (chunks not yet done are still found by their words). Content added before there's a
  provider waits, and is embedded once there is one.
- **Each bucket** has a title, a description of what belongs in it, an optional collection to follow, and limits: at
  most 2,000 documents (none for a bucket that follows a collection) and 50 MB of text. A document is at most 10 MB.

Turn the extension off on a single app with `search: {enabled: false}` in pocketone.yaml (on a host, an instance's
`extensions.search`); its collections and data stay.

## Use

**Buckets.** `$search.ensure({ name, title?, description?, owner?, source? })` makes a bucket if it's missing and
returns it; call it at startup for the bucket your app needs. An existing bucket keeps its settings (only an empty
title, description, owner or source is filled in), so your admins' changes stick. The description is what the Search
screen shows them: say what to load. A name is letters, digits, `-` and `_`, up to 64.

**Files and text.** Drop files or whole folders on a bucket's page, choose files, or paste text. From code,
`$search.add({ bucket, name, text })`; over HTTP, `POST /api/_c/search/v1/add` or the file itself as the body of
`POST /api/_c/search/v1/upload?bucket=<b>&name=<path>`. The name is the document's name in the bucket (usually its
path) and picks how it's split:

| Kind | Names | Split |
|---|---|---|
| Markdown | `.md`, `.mdx`, `.markdown` | At headings; each chunk carries its heading path ("Install › macOS", led by the front matter's `title`). Headings inside fenced code don't count. |
| Code | `.go`, `.js`, `.ts`, `.py`, `.sql`, `.yaml`, `.json` and other common source files | At blank lines. |
| Text | anything else | At blank lines. |

Paragraphs are packed into chunks of about 500 tokens, never over about 1,000. Only text is taken (binary files are
refused). Adding returns at once with the document waiting; a background worker embeds its chunks, 64 to a request and
under 120 requests a minute, and marks it `ready` (or `error`, with the reason; **Retry** on the screen splits and
embeds it again). It picks up where it left off after a restart.

Add a document with the same name to replace it. The same text again does nothing and costs nothing; a changed document
embeds only its chunks whose text changed. `$search.remove(bucket, names)` removes documents, `$search.reindex(bucket,
names?)` splits and embeds them again from the text kept with each document, and `$search.deleteBucket(name)` removes a
bucket and everything in it.

**Following a collection.** Give a bucket a `source` (on the screen: Settings → Follow a collection):

```ts
$search.ensure({ name: "notes", source: { collection: "notes", fields: ["title", "body"], filter: "published = true" } })
```

Each record the filter keeps becomes a document named `<collection>/<record id>`: the first field is its title, the
others its text (editor fields' HTML as plain text). Existing records are read when the bucket starts following;
after that, saving a record replaces its document (only edited text is embedded again), a record the filter no
longer keeps leaves the bucket, and deleting a record removes its document. Results from it name their `record`.

**Searching.** `await $search.searchAsync({ query, buckets?, limit?, prefix?, viewer? })` returns the best chunks across
the buckets named (every bucket when none are): `{ results: [{ bucket, document, text, heading, startLine, endLine,
record, score }], model, usage }`. `limit` is 5 by default, at most 50; `prefix` keeps documents whose name starts with
it. Each search embeds the query once (a few tokens; a repeated query is cached), the only part that calls a provider,
so it's the only async call. Ranking merges similarity of meaning (a loop in Go over vectors cached in memory) with
keyword matching (BM25), so exact names like `onRecordCreate` or a character's name rank where they should.

**Who sees what.** A bucket holds a copy of its records' text, so pass `viewer` to apply the collection's view rules:
`viewer: ctx.auth` (or `e.auth`) leaves out matches from records that user can't view, and `null` is a signed-out
guest. Without `viewer` a search sees everything, as your own server code. Over HTTP and MCP only superusers and API
tokens with the `search` scope (or `search:read` to list and search) get in.

**Enriching a model call.** `enrich` on `$llm.text.completeAsync`, `streamAsync` and LLM Chat's `sendAsync` (or a
chat's options) searches the buckets with the prompt and sends the best passages after it, numbered so the model can
cite them; `answer.passages` lists them. Give the buckets (`enrich: ["help"]`) or `{ buckets, limit?, query? }` (default
5 passages, at most 20). LLM Chat saves the passages with the question, so later turns send the same thing.

**Cost.** Embedding goes through llm.embed/v1, billed to your provider account like any other `$llm` call. Each
document shows what embedding it cost. `qwen3-embedding-4b` on Vultr is $0.05 per million tokens, about 4 MB of text.

## Integrations

| With | What happens |
|---|---|
| [LLM](../llm/README.md) | Embeds chunks and queries (llm.embed/v1), and adds passages to text calls with `enrich`. Search's own HTTP and MCP API is the [search/v1](../contracts/search/v1/README.md) contract. |
| [LLM Chat](../llm-chat/README.md) | `enrich` on a chat or a send. |
| [Key-Value Store](../kv/README.md) | Keeps the app's embedding model. |
| [Multisite](../multisite/README.md) | Each instance keeps its own buckets and model in its own database. |

## Example

```go
// Command example is a PocketBase app with search buckets: the llm extension embeds, search keeps the buckets. Add a
// provider with embedding models in Settings → LLM, run `go run . serve`, open the dashboard's Search screen, make a
// bucket and drop Markdown files on it. Or over HTTP as a superuser:
//
//	POST /api/_c/search/v1/ensure   {"name": "docs"}
//	POST /api/_c/search/v1/add      {"bucket": "docs", "name": "install.md", "text": "# Install\n..."}
//	POST /api/_c/search/v1/search   {"buckets": ["docs"], "query": "how do I install it"}
//
// In page code the same is $search.ensure(...), $search.add(...) and await $search.searchAsync(...).
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"

	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/llm"
	"pocketone.dev/ext/search"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app)
	if _, err := extkit.Setup(app, []extkit.Extension{llm.New(), search.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## License

MIT
