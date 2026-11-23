---
title: Answer from your content
description: "Buckets of content your app searches by meaning: files, folders, zips, web pages, Git repositories and collections, kept current; answer from them with enrich on $llm calls."
---

# Answer from your content

**Your app can search content you give it, and answer from it.** Fill a **bucket** from the dashboard: files, folders
and zips you drop, web pages and Git repositories it keeps current, or the records of your collections. Then name the bucket on a model call, and the passages
that best match the question go into the prompt with it, so the answer comes from your content instead of the model's
memory.

```ts
const answer = await $llm.text.completeAsync({
  model: "vultr/glm-5.3",
  system: "Answer from the passages given. Say so when they don't cover it.",
  prompt: "How do I export my notes?",
  enrich: ["help"],
})
// answer.text is the answer; answer.passages are the passages it was given
```

It uses the provider you added in Settings → LLM ([AI in your app](../index.md)), which needs embedding models
(Vultr's, for one). The model is picked for you and kept; you never name one.

## 1. Make a bucket

Your app makes the bucket it needs, once at startup. The description is what your admins see on the Search screen, so
say what belongs there:

```ts
// pb_hooks/search.pb.js
onBootstrap((e) => {
  e.next()
  $llm.search.ensure({
    name: "help",
    title: "Help center",
    owner: "Help desk",
    description: "Our help articles, as Markdown. The help desk answers from these.",
  })
})
```

Calling it again changes nothing, so your admins' changes stick. You can also make a bucket on the Search screen.

## 2. Fill it

Open the dashboard and choose **Search** in the header. A sidebar lists **Ask**, **Search**, **Settings** and every
bucket. Open yours and add **sources**. Each one is a single row you can refresh, replace or remove, however many documents it brings:

- **Drop files, folders or zips** on it (Markdown, text or code inside), or choose files.
- **Paste text.**
- **Add a URL or Git repo**: paste any URL. Its server is asked whether it's a Git repository (with or without `.git`,
  on any host), and the form shows how it will be read, with a switch if you want the other. Pick how often to read it
  again: every hour, day or week. A repository brings its default branch's text files and its recent commits.
- **Follow a collection**: pick the collection, check the fields to search (its text fields start checked; the first
  is each record's title) and, if you like, which records (`published = true`). It reads the records there now and
  keeps up as they're saved and deleted.

Dropping something with the same name as a source replaces it: files that didn't change cost nothing, and files it no
longer has are removed. For a new version under another name (`guide-v2.zip`), use **Upload a new version** on the old
one's row. Watch each source go from `reading` to `ready` and its documents from `in line` to `embedding` to `ready`;
open a row to see its documents.

To check what's in a bucket, use **Ask** in the sidebar: pick buckets and a model, ask a question in words, and read
the answer with the passages it came from (a follow-up keeps the conversation). **Search** shows the raw matching
passages instead: what `$llm.search` and `enrich` get. **Settings** shows the embedding model and how far embedding has got.

Nothing is embedded twice: the same file in two buckets, or two buckets following one collection, cost one embedding.

### Fill it from code

Everything the screen does, your code can do with `$llm.search`. Adding returns at once; the content is read and
embedded in the background.

```ts
// a file's text, or text you put together (a source of one document; the same name again replaces it)
$llm.search.add({ bucket: "help", name: "export.md", text: "# Exporting\n\nChoose File → Export…" })

// a web page or a Git repository, read again every day
$llm.search.addURL({ bucket: "help", url: "https://github.com/acme/help-articles", every: "day" })

// a collection: a document per published article, kept in sync as records are saved and deleted
$llm.search.follow({ bucket: "help", collection: "articles", fields: ["title", "body"], filter: "published = true" })
```

Then look at what's there, and tidy up:

```ts
const sources = $llm.search.sources("help")          // [{ name, kind, status, documents, pending, error, ... }]
const docs = $llm.search.documents({ bucket: "help", source: "export.md" })
$llm.search.refresh("help", "https://github.com/acme/help-articles") // read it again now
$llm.search.remove("help", ["export.md"])            // take sources out, with their documents
const { model, chunks, embedded } = $llm.search.status() // embedded < chunks: still embedding
```

A source's name is how it's found again: the file name you gave, the URL, or the collection. Adding one of the same
name replaces it, and unchanged text isn't embedded again.

## 3. Answer from it

`enrich` works on `$llm.text.completeAsync`, `$llm.text.streamAsync` and LLM Chat:

```ts
const chat = $llm.chat.open(id, { model: "vultr/glm-5.3", system: preamble, enrich: ["help"] })
const answer = await chat.sendAsync("And as PDF?")
```

Give several buckets to search them together (`enrich: ["help", "changelog"]`), or `{ buckets, limit }` for more or
fewer passages (5 by default). To show results yourself, search directly:

```ts
const found = await $llm.search.searchAsync({ buckets: ["help"], query: "export notes", limit: 5 })
// found.results: [{ bucket, document, heading, text, record, score }, ...], best first
```

## Records and who sees them

A collection source holds a copy of its records' text, so searching it skips the collection's API
rules unless you say who's asking. Pass the signed-in user as `viewer` and matches from records they can't view are
left out (`null` is a signed-out visitor):

```ts
const found = await $llm.search.searchAsync({ buckets: ["notes"], query, viewer: ctx.auth })
```

The same goes for `enrich` on a model call: `enrich: { buckets: ["notes"], viewer: ctx.auth }`. Without `viewer`, a
search sees everything, as your own server code does.

## Changing the model

**Settings** on the Search screen shows the app's embedding model. Change it there and every bucket is embedded again in the
background, with its progress on the screen. Search keeps working meanwhile; parts not yet re-embedded are still found
by their words.

## What it costs

Embedding is billed to your provider account like any `$llm` call: `qwen3-embedding-4b` on Vultr is $0.05 per million
tokens, about 4 MB of text. Each document shows what it cost. Each search embeds only the question, a few tokens.

## `$llm.search` at a glance

Every call is local and returns at once, except `searchAsync`, which asks your embedding model about the query.

| Call | What it does |
|---|---|
| `buckets()` | Every bucket, with its counts. |
| `bucket(name)` | One bucket. |
| `ensure({ name, title?, description?, owner? })` | Makes the bucket if it's missing and returns it; an existing one keeps its settings. |
| `update({ name, title?, description?, maxDocuments?, maxBytes? })` | Changes a bucket. |
| `deleteBucket(name)` | Deletes a bucket and everything in it. |
| `sources(bucket)` | Its sources: `{ name, kind, status, documents, chunks, embedded, pending, error, refreshed, next, ... }`. |
| `add({ bucket, name, text, replace? })` | A file or text as a source; `replace` names an older source a renamed version replaces. |
| `addURL({ bucket, url, every?, kind?, commits?, exclude? })` | A web page or Git repository (`kind` is decided by asking the server); `every` is `hour`, `day` or `week`. |
| `follow({ bucket, collection, fields, filter? })` | A collection; the first field is each record's title. |
| `refresh(bucket, source)` | Reads a source again now. |
| `remove(bucket, sources)` | Takes sources out, with their documents. |
| `documents(bucket \| { bucket, source?, prefix? })` | Documents, with their status and how many chunks are embedded. |
| `status()` | The app's embedding model, and how many chunks are embedded of how many. |
| `await searchAsync({ query, buckets?, limit?, prefix?, viewer? })` | The best passages across the buckets (all when none are named): `{ results: [{ bucket, source, document, heading, text, record, score }], model, usage }`. |

The same operations are the [llm.search/v1](../../../contracts/llm/search/v1/README.md) contract: over HTTP at
`/api/_c/llm.search/v1/…` (a superuser, or an API token with the `search` scope), from Go with
`searchv1.Contract.Must(app)`, and as MCP tools. The [LLM Search extension](../../../llm-search/README.md) has the
details: what's skipped in folders, zips and repositories, the limits, and how it ranks.
