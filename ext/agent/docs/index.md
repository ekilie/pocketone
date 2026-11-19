---
title: AI in your hooks
section: New in PocketOne
description: "$agent gives your pb_hooks and pages a language model: label records, pull JSON out of text, summarize, answer questions. Built in, on your own CPU, or through any OpenAI-compatible API."
---

# AI in your hooks

**Your hooks can ask a language model.** `$agent.complete()` sends a prompt and gives you the answer, from inside any
hook or page:

```js
const r = $agent.complete({ prompt: "Summarize in one sentence: " + record.get("body"), maxTokens: 60 })
// r.text is the summary
```

Use it for the small jobs that come up around your data: label a support ticket, pull an address out of an email,
summarize a long post, write a title or an alt text, answer a question about your docs.

It's built into `pocketone`. The model can be one PocketOne runs itself, on your server's CPU with nothing else to
install, or any OpenAI-compatible API (OpenAI, OpenRouter, Groq, or Ollama on your own machine). Your code is the same
either way.

## Try it in two minutes

1. Pick a model. In the dashboard, open **Settings → LLM**, choose **Built-in** and `qwen3-1.7b`, and save. The first
   time, PocketOne downloads it (1.8 GB) and shows the progress. Or add it to `pocketone.yaml`:

   ```yaml
   agent:
     model: qwen3-1.7b
   ```

2. Label support tickets as they come in. Make a `tickets` collection with two text fields, `body` and `label`, and add
   a hook:

   ```js
   // pb_hooks/tickets.pb.js
   onRecordCreate((e) => {
     const r = $agent.complete({
       system: "Label the support ticket.",
       prompt: e.record.get("body"),
       schema: { type: "object", properties: { label: { enum: ["bug", "billing", "question"] } }, required: ["label"] },
       maxTokens: 30,
     })
     e.record.set("label", r.data.label)
     e.next()
   }, "tickets")
   ```

3. Create a ticket that says "I was charged twice this month". Its `label` is `billing`.

The `schema` makes the model answer with JSON in exactly that shape, and `r.data` is the parsed value: see
[Structured answers](guides/structured.md).

## What's next

- [Choosing a model](choosing.md): built in or your own API, which model, and how fast each is.
- [Structured answers](guides/structured.md): JSON you can rely on, even from a small model.
- [Streaming answers](guides/streaming.md): show an answer as it's written, in a page.
- [Your own knowledge](guides/context.md): answer questions about your docs or data.
- [On a multisite host](hosting.md): one model shared by every app on a server.

The API is [agent/v1](../../contracts/agent/v1/README.md); the extension's reference is [its README](../README.md).
