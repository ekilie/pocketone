---
title: "Building Docs Chat: a docs bot in five files"
description: How learn.pocketone.dev works, file by file. A PocketOne app with no Go that answers from the PocketOne and PocketBase docs, streams its answer and shows what it cost.
date: 2026-11-22
author: Ben Allfree
tags: examples, AI, Pages, llm
---

Docs Chat started as a test. I wanted to see how inference clouds actually perform when you call them from a PocketOne app, and I needed something real to exercise the new inference interface in the JSVM engine. Then I got curious. What would a simple chatbot look like that could teach you something about PocketOne?

That's [learn.pocketone.dev](https://learn.pocketone.dev). Ask it something about PocketOne or PocketBase and it answers from both sets of docs, cites the pages it used, and tells you what the answer cost.

![Docs Chat answering "How do I restore my database to an earlier moment?": dashboard steps, the CLI command, a few things to know, then the model, time, tokens, cost and the docs sections it used](/blog/img/docs-chat-answer.jpg)

It's five files. No Go, no build step, no vector database. Just a `pb_pages` folder and the stock `pocketone` binary. The whole thing is in the repo at [`ext/llm/examples/chat`](https://github.com/pocketonehq/pocketone/tree/main/ext/llm/examples/chat).

```
pb_pages/
  index.ejs              the chat page
  chat/+api.ts           POST /chat: the answer as server-sent events
  chat/suggest/+api.ts   POST /chat/suggest: three follow-up questions
  _private/llm.ts        the model, the docs, search, the call to the chat
  _private/preamble.md   who the assistant is and how it answers
```

### Teaching it the docs

The first version answered from copies of the extension READMEs. That was the wrong material. People don't ask "what does the `llm.align/v1` contract promise," they ask "how do I deploy." So now it reads the same thing an AI agent would: pocketone.dev's [`llms.txt`](https://pocketone.dev/llms.txt) and `llms-full.txt`. It keeps the user docs (Get started, New in PocketOne, Guides, Pages) and skips the extension and contract reference.

Then I gave it PocketBase's own docs too. PocketOne keeps a Markdown copy of pocketbase.io's docs in the pocketone Agent Skill, so `_private/llm.ts` reads those as well. Every excerpt is labelled "PocketOne docs" or "PocketBase docs" and linked back to its site, and `preamble.md` explains how the two fit together.

Both sets load in the background and refresh every ten minutes. They're split into sections, and the words in each section are indexed.

### The docs don't fit

My first idea was the obvious one. Put all the docs in the preamble and let the model read everything. That didn't work. GLM, the model I was using, couldn't fit the PocketOne and PocketBase docs in its context.

So I added a pre-filter. Before the model sees anything, the app searches the docs and sends only the sections that are hopefully relevant.

If I were building a real chatbot, I'd do it the other way. I'd use a bigger model, put the entire docs in the preamble, and let the provider cache it. But as near as I can tell, those caches aren't held forever. Every time the cache goes cold, the whole preamble gets sent again. That means more tokens at startup, and a slower first answer for whoever shows up after a quiet spell. I didn't try it, so that's my best guess and not a measurement. For proving the point, the pre-filter was good enough.

### Finding the right excerpts

No embeddings. Each question gets a plain BM25 search over those sections, with a few synonyms thrown in (people say "apps," the docs say "instances"). The top three sections go to the model.

A follow-up like "and how do I undo that?" has no topic of its own, so the search also includes the question before it:

```ts
const before = chatId
  ? $llm.chat.open(id).history({ limit: 2 }).filter((m) => m.role === 'user').map((m) => m.content)
  : []
const context = search(kb, [...before, question].join(' '), 3)
```

Sending only three excerpts keeps each question small and cheap. That matters on a public page anyone can hammer.

### Asking the model

This is where `$llm.chat` does the heavy lifting. The page holds nothing but a chat id. The chat keeps the conversation, sends the preamble and the turns so far, and puts this question's excerpts after it:

```ts
const chat = $llm.chat.open(id, {
  model: 'vultr/glm-5.3',
  system: kb.preamble,
  maxTokens: 600,
  temperature: 0.3,
  reasoning: 'off',
})
const res = await chat.sendAsync(question, {
  context: `Documentation excerpts to answer from:\n\n${excerpts}`,
  onText,
})
```

Reasoning is off. The excerpts already hold the answer, and thinking would only cost time.

There's a nice side effect of putting the excerpts after the question and keeping the question as sent. Each turn's prompt starts exactly like the last one did, so providers bill that repeated start at their cheaper cached rate.

### Streaming it to the browser

`chat/+api.ts` is the whole server side of the chat. It opens a server-sent events stream and forwards each piece of the answer as the model writes it:

```ts
export async function post(ctx: any) {
  const { chat, question } = ctx.req.body()
  const stream = ctx.res.sse()
  try {
    const r = await ctx.resolve('llm').answer(ctx, {
      chatId: chat,
      question,
      onText: (text: string) => stream.send('text', JSON.stringify(text)),
    })
    stream.send('done', JSON.stringify(r))
  } catch (err: any) {
    stream.send('error', JSON.stringify(String(err.message || err)))
  }
  stream.close()
}
```

The `done` event carries the chat id, the sources and the cost. Because the call is awaited in an async handler, the model works off the app's JS VM. The app keeps serving everyone else while one answer streams.

### Follow-up questions

After each answer, `chat/suggest/+api.ts` asks for three follow-up questions. This one uses a JSON schema, so the reply comes back as data and not prose I'd have to parse:

```ts
const res = await $llm.text.completeAsync({
  model: MODEL,
  reasoning: 'off',
  system: 'Suggest three short follow-up questions a PocketOne user might ask next. Each under eight words.',
  prompt: `Question: ${question}\nAnswer: ${reply}`,
  schema: {
    type: 'object',
    properties: { questions: { type: 'array', items: { type: 'string' }, minItems: 3, maxItems: 3 } },
    required: ['questions'],
    additionalProperties: false,
  },
})
return res.data.questions
```

### What it cost

Every `$llm` result has `usage.costMicroUSD`, the call's cost in millionths of a dollar. It's the provider's figure when they report one, else estimated from the model's listed prices. The page shows it under every answer, so you can see exactly what a question costs. The restore answer above took 1.6 seconds and cost about $0.0008, and 512 of its 682 input tokens were billed at the cached rate.

<p style="display: flex; justify-content: center"><img src="/blog/img/docs-chat-phone.jpg" alt="The same answer on a phone, with the cost line and sources under it" width="300"></p>

### The `$llm` interface

Docs Chat is small. Most of my time went into what's underneath it: the `$llm` object. I wanted one interface that works the same across providers, so switching from Vultr to Cloudflare Workers AI to OpenRouter is a change to the model name and nothing else. Every call names its model as `provider/model`. Every call that reaches a provider is async and says so in its name (`completeAsync`, `sendAsync`). Every result has the same shape, cost included.

I spent a lot of time fine-tuning that interface, and Docs Chat was the app I tuned it against. The goal is simple. Writing an inference-powered agent into a PocketOne app should be easy. If it took me five files to build a docs bot, it should take you about the same to build whatever you have in mind. The full API is in [AI in your app](/docs/extensions/llm).

### Run it yourself

You need a provider key (Vultr, Cloudflare Workers AI, OpenRouter or any OpenAI-compatible API).

```bash
cd ext/llm/examples/chat
pocketone superuser upsert admin@example.com password --dir=pb_data
pocketone serve --dir=pb_data
```

Open the dashboard at `/_/`, add your provider in Settings → LLM, hit Test, and open the app. If your provider isn't Vultr, set `CHAT_MODEL` to a `provider/model` you have.

The live copy runs on PocketHost. Shipping a change is `pocketone deploy --restart` from the example's folder.

The [Docs Chat walkthrough](/docs/examples/docs-chat) in the docs covers the same ground in reference form, and [Example apps](/docs/examples) has the rest. Next up, [the bedtime story app](/blog/building-bedtime-story), which is a lot sillier and a lot more involved.
