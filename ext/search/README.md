---
owners: ["@benallfree"]
version: 0.1.0
title: "Search"
group: binary
summary: "Buckets of content your app searches by meaning and keywords: $search.ensure, $search.searchAsync, and a dashboard screen to load them."
---

# Search

Search keeps **buckets** of content your app searches. Your app makes the bucket it needs and searches it; you load the
content into it from the dashboard's **Search** screen, by dropping files or pasting text. Each document is split into
chunks that remember where they came from (the document, its section, its lines), embedded through the
[LLM](../llm/README.md) extension, and ranked by meaning and by keywords together. Use it to ground a model's answer in
your own content: search first, then send the best chunks with the question.

```ts
$search.ensure({ name: "storytime", title: "Story Time library", description: "Stories, characters and places to draw on." })

const found = await $search.searchAsync({ bucket: "storytime", query: "a fox who lives by the sea", limit: 5 })
for (const r of found.results) console.log(r.document, r.heading, r.score, r.text)
```

In Go:

```go
s := searchv1.Contract.Must(app) // search/v1, from pocketone.dev/ext/contracts/search/v1
s.Ensure(ctx, extkit.Internal, searchv1.EnsureInput{Name: "storytime", Title: "Story Time library"})
res, err := s.Search(ctx, extkit.Internal, searchv1.SearchInput{Bucket: "storytime", Query: "a fox who lives by the sea"})
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

Each bucket has its own settings, on the Search screen or through `$search.update`:

| Setting | Default | What it does |
|---|---|---|
| Embedding model | none | A `provider/model` from Settings → LLM, e.g. `vultr/qwen3-embedding-4b`. Without one the bucket is searched by keywords only and nothing is embedded. Changing it embeds every chunk again (the screen says how many tokens first). |
| Dimensions | the model's | Shortens the vectors, for models that allow it (1024 keeps `qwen3-embedding-4b` at 4 KB a chunk). |
| Most documents | 2,000 | Adding past it is refused. |
| Most text | 50 MB | The same, for the bucket's total size. |

A document is at most 10 MB of text. Turn the extension off on a single app with `search: {enabled: false}` in
pocketone.yaml (on a host, an instance's `extensions.search`); its collections and data stay.

## Use

**Buckets.** `$search.ensure({ name, title?, description?, owner?, model?, dimensions? })` makes a bucket if it's
missing and returns it; call it at startup for the bucket your app needs. An existing bucket keeps its settings (only an
empty title, description or owner is filled in), so your admins' choices stick. The description is what the Search
screen shows them: say what to load. A name is letters, digits, `-` and `_`, up to 64.

**Adding content.** Drop files or whole folders on a bucket's page, choose files, or paste text. From code,
`$search.add({ bucket, name, text })`; over HTTP, `POST /api/_c/search/v1/add` or the file itself as the body of
`POST /api/_c/search/v1/upload?bucket=<b>&name=<path>`. The name is the document's name in the bucket (usually its
path) and picks how it's split:

| Kind | Names | Split |
|---|---|---|
| Markdown | `.md`, `.mdx`, `.markdown` | At headings; each chunk carries its heading path ("Install › macOS", led by the front matter's `title`). Headings inside fenced code don't count. |
| Code | `.go`, `.js`, `.ts`, `.py`, `.sql`, `.yaml`, `.json` and other common source files | At blank lines. |
| Text | anything else | At blank lines. |

Paragraphs are packed into chunks of about 500 tokens, never over about 1,000. Only text is taken (binary files are
refused); PDF and web pages come later.

Adding returns at once with the document `queued`: a background worker embeds its chunks, 64 to a request and under 120
requests a minute, and marks it `ready` (or `error`, with the reason; the screen's **Retry** splits and embeds it
again). It picks up where it left off after a restart.

**Changing content.** Add a document with the same name to replace it. The same text again does nothing (and costs
nothing); a changed document embeds only its chunks whose text changed, reusing the rest. `$search.remove(bucket,
names)` removes documents, `$search.reindex(bucket, names?)` splits and embeds them again from the text kept with each
document, and `$search.deleteBucket(name)` removes a bucket and everything in it.

**Searching.** `await $search.searchAsync({ bucket, query, limit?, mode?, prefix? })` returns the best chunks, best
first: `{ results: [{ text, document, heading, startLine, endLine, score, semantic, keyword }], mode, model, usage }`.

| Mode | Ranks by |
|---|---|
| `hybrid` (default) | Meaning and keywords together (reciprocal rank fusion). Keywords catch exact names like `onRecordCreate` that embeddings blur; meaning catches a reworded question. If the model can't embed the query, it ranks by keywords alone and `note` says why, so a page that grounds its answer in the bucket keeps working. |
| `semantic` | Cosine similarity to the query's embedding. |
| `keyword` | BM25 over the chunks' text and headings. No model call. |

`limit` is 5 by default, at most 50. `prefix` keeps chunks from documents whose name starts with it (`"guides/"`).
Searching embeds the query once (a few tokens; repeated queries are cached), the only part that reaches a provider, so
it's the only async call. Vectors are ranked by a loop in Go over the bucket's vectors, cached in memory: thousands of
chunks take milliseconds.

**Who can do what.** Over HTTP and MCP only superusers and API tokens with the `search` scope (or `search:read` to list
and search) get in. Your users reach a bucket only through your own page code, which decides what they may search.

**Cost.** Embedding goes through llm.embed/v1, so it's billed to your provider account like any other `$llm` call.
Each document shows what embedding it cost. `qwen3-embedding-4b` on Vultr is $0.05 per million tokens, about 4 MB of text.

## Integrations

| With | What happens |
|---|---|
| [LLM](../llm/README.md) | Embeds the chunks and queries (llm.embed/v1). Without it, buckets are searched by keywords only. Its HTTP and MCP API is the [search/v1](../contracts/search/v1/README.md) contract. |
| [LLM Chat](../llm-chat/README.md) | Pass the search results as a send's `context` to answer from your content. |
| [Multisite](../multisite/README.md) | Each instance keeps its own buckets in its own database. |

## Example

```go
// Command example is a PocketBase app with search buckets: the llm extension embeds, search keeps the buckets. Add a
// provider in Settings → LLM, run `go run . serve`, open the dashboard's Search screen, make a bucket with an
// embedding model ("vultr/qwen3-embedding-4b") and drop Markdown files on it. Or over HTTP as a superuser:
//
//	POST /api/_c/search/v1/ensure   {"name": "docs", "model": "vultr/qwen3-embedding-4b", "dimensions": 1024}
//	POST /api/_c/search/v1/add      {"bucket": "docs", "name": "install.md", "text": "# Install\n..."}
//	POST /api/_c/search/v1/search   {"bucket": "docs", "query": "how do I install it"}
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
