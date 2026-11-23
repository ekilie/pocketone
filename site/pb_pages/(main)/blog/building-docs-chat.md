---
title: "From Docs Chat to the ask starter kit"
description: Docs Chat was my first app on $llm. What it taught me is now pocketone.dev's ask box and the ask starter kit, so an ask-your-docs app is one command away.
date: 2026-11-22
author: Ben Allfree
tags: examples, AI, Pages, llm
---

Docs Chat started as a test. I wanted to see how inference clouds actually perform when you call them from a PocketOne app, and I needed something real to exercise the new inference interface in the JSVM engine. Then I got curious about what a simple chatbot would look like that could teach you something about PocketOne.

It ran for a little while at learn.pocketone.dev, and I've since retired it. pocketone.dev has an ask box in the docs header now, built on LLM Search, and it does the same job better and in the place people already are. So the idea moved somewhere more useful: a starter kit you can point at your own docs.

```bash
pocketone pages create mydocs --starter ask
```

![The ask starter kit: a question box over the docs](/docs/screenshots/starter-ask.jpg)

### What's in it

It's a `pb_pages` folder and the stock `pocketone` binary. No Go, no build step and no vector database to run.

```
pb_pages/
  +layout.ets        the page shell, with links to Ask and Docs
  index.ets          the question form and the answer, written on the server
  _private/ask.ts    the docs bucket, the search and the model call
  docs/              your docs as Markdown pages
```

The docs are ordinary Markdown pages in `pb_pages/docs/`, so you can browse them on their own, and they're also what the answers draw on. The kit comes with three sample pages for you to replace.

### Why it doesn't send all the docs

My first idea with Docs Chat was the obvious one. Put all the docs in the preamble and let the model read everything. That didn't work. GLM, the model I was using, couldn't fit the PocketOne and PocketBase docs in its context. So I added a pre-filter: search the docs first and send the model only the sections that are hopefully relevant.

If I were building a big chatbot I might do it the other way, with a bigger model and the whole docs in a preamble the provider caches. But as near as I can tell, those caches aren't held forever, and every time one goes cold the whole preamble gets sent again. I didn't try it, so that's my best guess and not a measurement. For proving the point, searching first was good enough, and it still is.

### Finding the right passages

In Docs Chat the search was a plain BM25 keyword search I wrote by hand, with a few synonyms thrown in because people say "apps" and the docs say "instances". It worked well enough, and it was also the kind of thing every app like this ends up writing again. So it became part of PocketOne. LLM Search keeps buckets of your content, splits them into chunks, embeds them once, and searches by keywords and by meaning at the same time.

In the kit that's two calls. Make a bucket and point it at the docs folder, and LLM Search reads it again whenever the files change:

```ts
$llm.search.ensure({ name: 'docs', title: 'Docs' })
$llm.search.addFolder({ bucket: 'docs', path: 'pb_pages/docs', include: ['*.md'], url: '/docs/{path}' })
```

Each question is then one search, and the best five passages come back with the page and section they came from:

```ts
const found = await $llm.search.searchAsync({ buckets: 'docs', query: question, limit: 5 })
```

### Asking the model

The passages go to the model numbered, and the instructions ask it to answer only from them and cite them as [1], [2]. The call goes through `$llm.for(ctx)`, so it's counted for whoever asked, their account or a guest cookie, and the limits in Settings → LLM apply to them:

```ts
const res = await $llm.for(ctx).text.completeAsync({
  model: MODEL,
  system: SYSTEM,
  prompt: `Passages:\n\n${passages}\n\nQuestion: ${question}`,
  maxTokens: 400,
  temperature: 0.2,
})
```

That part matters on a public page anyone can hammer. A visitor who isn't signed in gets no answer until you've set a limit for all guests together, so a copy you put online can't run up a bill by accident.

The page is written on the server, so it works without JavaScript and every question is a link (`/?q=...`). The sources are listed under the answer, linked to their pages.

### What it costs

Every `$llm` result has `usage.costMicroUSD`, the call's cost in millionths of a dollar, and every call is kept in the LLM ledger, so Settings → LLM shows what today and this month have cost. Docs Chat showed the cost under every answer. A typical one took about 1.6 seconds and cost about $0.0008.

### The `$llm` interface

Most of my time on Docs Chat went into what's underneath it: the `$llm` object. I wanted one interface that works the same across providers, so switching from Vultr to Cloudflare Workers AI to OpenRouter is a change to the model name and nothing else. Every call names its model as `provider/model`. Every call that reaches a provider is async and says so in its name (`completeAsync`, `searchAsync`). Every result has the same shape, cost included.

Docs Chat was the app I tuned that interface against, and the goal hasn't changed. Writing an inference-powered feature into a PocketOne app should be easy. The full API is in [AI in your app](/docs/extensions/llm).

### The bigger version

pocketone.dev's own ask box is the same idea with more on it. It streams the answer as it's written, keeps the answers to repeat questions for an hour, and answers in the code language you picked on the docs' TS and Go tabs. Its source is [`site/pb_pages/_private/ask.ts`](https://github.com/pocketonehq/pocketone/blob/main/site/pb_pages/_private/ask.ts), if you want to see where the kit can go.

### Run it yourself

You need a provider with chat and embedding models (Vultr has both).

```bash
pocketone pages create mydocs --starter ask
cd mydocs
pocketone superuser upsert admin@example.com password --dir ./pb_data
pocketone serve --dir ./pb_data
```

Open the dashboard at `/_/`, add your provider in Settings → LLM along with a limit for all guests together, then open the app and ask something. Swap the sample pages in `pb_pages/docs/` for your own when you're ready.

The [ask starter kit](/docs/starters/ask) page has the rest. Next up, [the bedtime story app](/blog/building-bedtime-story), which is a lot sillier and a lot more involved.
