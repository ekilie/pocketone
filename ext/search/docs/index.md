---
title: Search your content
section: New in PocketOne
description: "Buckets of content your app searches by meaning: drop files on them in the dashboard or have them follow a collection, then answer from them with enrich on $llm calls."
---

# Search your content

**Your app can search content you give it, and answer from it.** Put documents in a **bucket**: files you drop on it
in the dashboard, or the records of one of your collections. Then name the bucket on a model call, and the passages
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

You need an AI provider with embedding models in Settings → LLM (Vultr's, for one). Search picks the model and keeps it;
you never name one.

## 1. Make a bucket

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
  })
})
```

Calling it again changes nothing, so your admins' changes stick. You can also make a bucket on the Search screen.

## 2. Fill it

Open the dashboard and choose **Search** in the header. Every bucket is listed with what's in it. Open yours, then:

- **Drop files or whole folders** on it (Markdown, text or code), choose files, or paste text.
- Or open **Settings** and have it **follow a collection**: pick the collection, the fields to search (the first is each
  record's title) and, if you like, which records (`published = true`). It reads the records there now and keeps up as
  they're saved and deleted.
- Watch each document go from `waiting` to `ready` as it's embedded. A failed one shows why, with **Retry**.
- **Try a search** to see what comes back, with each match's document, section and lines.

Dropping a file with a name that's already there replaces it. The same file again costs nothing, and a changed one
embeds only the parts that changed.

## 3. Answer from it

`enrich` works on `$llm.text.completeAsync`, `$llm.text.streamAsync` and LLM Chat:

```ts
const chat = $llm.chat.open(id, { model: "vultr/glm-5.3", system: preamble, enrich: ["help"] })
const answer = await chat.sendAsync("And as PDF?")
```

Give several buckets to search them together (`enrich: ["help", "changelog"]`), or `{ buckets, limit }` for more or
fewer passages (5 by default). To show results yourself, search directly:

```ts
const found = await $search.searchAsync({ buckets: ["help"], query: "export notes", limit: 5 })
// found.results: [{ bucket, document, heading, text, record, score }, ...], best first
```

## Records and who sees them

A bucket that follows a collection holds a copy of its records' text, so searching it skips the collection's API
rules unless you say who's asking. Pass the signed-in user as `viewer` and matches from records they can't view are
left out (`null` is a signed-out visitor):

```ts
const found = await $search.searchAsync({ buckets: ["notes"], query, viewer: ctx.auth })
```

Without `viewer`, a search sees everything, as your own server code does.

## Changing the model

The Search screen shows the app's embedding model. Change it there and every bucket is embedded again in the
background, with its progress on the screen. Search keeps working meanwhile; parts not yet re-embedded are still found
by their words.

## What it costs

Embedding is billed to your provider account like any `$llm` call: `qwen3-embedding-4b` on Vultr is $0.05 per million
tokens, about 4 MB of text. Each document shows what it cost. Each search embeds only the question, a few tokens.

The full reference, with every operation over HTTP and Go, is the [Search extension](../README.md).
