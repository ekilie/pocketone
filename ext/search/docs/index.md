---
title: Search your content
section: New in PocketOne
description: "Buckets of content your app searches by meaning and keywords: your app makes the bucket, you drop files on it in the dashboard, and $search finds the passages that answer a question."
---

# Search your content

**Your app can search content you give it.** Put documents in a **bucket**, and `$search` finds the passages that best
match a question, by meaning and by keywords together. The usual use is grounding a model's answer: search first, then
send the best passages with the question, so the answer comes from your content instead of the model's memory.

```ts
const found = await $search.searchAsync({ bucket: "help", query: "how do I export my notes?", limit: 4 })
const context = found.results.map((r) => `From ${r.document} (${r.heading}):\n${r.text}`).join("\n\n")
const answer = await $llm.text.completeAsync({ model: "vultr/glm-5.3", system: "Answer from the passages given.", prompt: question + "\n\n" + context })
```

## 1. Make a bucket in your app

Your app makes the bucket it needs, once at startup. The description is what your admins see on the Search screen, so
say what belongs there:

```ts
// pb_hooks/search.pb.js
onBootstrap((e) => {
  e.next()
  $search.ensure({
    name: "help",
    title: "Help center",
    owner: "Help desk",
    description: "Our help articles, as Markdown. The help desk answers from these.",
    model: "vultr/qwen3-embedding-4b",
    dimensions: 1024,
  })
})
```

Calling it again changes nothing, so your admins' settings stick. Without a `model` the bucket is searched by keywords
only, which costs nothing and needs no provider.

## 2. Load it in the dashboard

Open the dashboard and choose **Search** in the header. Every bucket is listed with what's in it. Open yours, then:

- **Drop files or whole folders** on it (Markdown, text or code), or choose files, or paste text.
- Watch each document go from `queued` to `ready` as it's embedded. A failed one shows why, with **Retry**.
- **Try a search** to see what comes back, with each match's document, section, lines and score.
- Remove documents, change the bucket's model or limits under **Settings**, or delete it.

Dropping a file with a name that's already there replaces it. The same file again costs nothing, and a changed one
embeds only the parts that changed.

## 3. Search it

```ts
const r = await $search.searchAsync({ bucket: "help", query: "export notes", limit: 5 })
// r.results: [{ text, document, heading, startLine, endLine, score }, ...], best first
```

Markdown is split at its headings, so each result says which section it's from (`"Export › As PDF"`). Searching is
async because it asks the embedding model about the query; everything else in `$search` (`ensure`, `add`, `documents`,
`remove`) is local and plain.

To fill a bucket from code instead, `$search.add({ bucket, name, text })` adds or replaces a document; it's embedded in
the background.

## What it costs

Embedding is billed to your provider account like any `$llm` call: `qwen3-embedding-4b` on Vultr is $0.05 per million
tokens, about 4 MB of text. Each document shows what it cost. Each search embeds only the query, a few tokens.

The full reference, with every operation over HTTP and Go, is the [Search extension](../README.md).
