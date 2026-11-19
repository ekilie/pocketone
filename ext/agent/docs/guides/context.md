---
title: Your own knowledge
description: "Answer questions about your docs or data: arm $agent with a preamble and the passages that match each question, read from files or records."
---

# Your own knowledge

A model knows a lot in general and nothing about your app. To answer questions about your docs, products or records,
give it the relevant text with each question. Two parts do it:

- **A preamble:** who the assistant is and how it answers ("answer from the excerpts below; say you don't know rather
  than guess").
- **Context:** the few passages that match the question, picked from your text each time.

Picking matters: a small model on a CPU reads a few hundred words in a second or two, not your whole manual. Search
first, then send only what matches.

## From files on disk

Keep the preamble and your docs as Markdown files in `pb_pages/_private/` (never served), and read them from page code:

```js
// pb_pages/_private/assistant.js
const dir = (ctx) => `${ctx.pagesDir}/_private`
const read = (path) => toString($os.readFile(path))

function knowledge(ctx) {
  const files = $os.readDir(`${dir(ctx)}/context`).filter((e) => e.name().endsWith(".md"))
  return {
    preamble: read(`${dir(ctx)}/preamble.md`),
    docs: files.map((e) => ({ name: e.name(), text: read(`${dir(ctx)}/context/${e.name()}`) })),
  }
}

// a simple search: the documents sharing the most words with the question
function search(kb, question, n) {
  const words = new Set(question.toLowerCase().split(/[^a-z0-9]+/).filter((w) => w.length > 2))
  return kb.docs
    .map((d) => ({ d, score: d.text.toLowerCase().split(/[^a-z0-9]+/).filter((w) => words.has(w)).length }))
    .filter((x) => x.score > 0)
    .sort((a, b) => b.score - a.score)
    .slice(0, n)
    .map((x) => x.d)
}

function answer(ctx, question) {
  const kb = knowledge(ctx)
  const system = [kb.preamble, ...search(kb, question, 2).map((d) => `[${d.name}]\n${d.text}`)].join("\n\n")
  return $agent.complete({ system, prompt: question, maxTokens: 300, temperature: 0.3 })
}

module.exports = { answer }
```

A page calls it with `ctx.resolve("assistant").answer(ctx, question)`. Split long documents into sections at their
headings so a match brings back one section, not a whole page.

## From records

The same works with your data: find the records that match (a filter, or a full-text search you keep in a field),
format the useful fields as text, and pass them as the context. Only pass what the person asking is allowed to see.

## A complete example

[examples/chat](../../examples/chat/README.md) is a chat app about PocketOne built this way, as a PocketPages app with no
Go: eleven docs pages as context, split into sections and ranked with BM25 (with a few synonyms, since people say
"apps" where the docs say "instances"), answers streamed as they're written, and three follow-up questions suggested
with a schema.
