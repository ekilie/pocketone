---
title: Docs Chat
description: A chat about PocketOne and PocketBase that answers from both sets of docs, streams its answer and shows what it cost - how it's built, file by file.
---

# Docs Chat

Try it at [learn.pocketone.dev](https://learn.pocketone.dev). Source:
[`ext/llm/examples/chat`](https://github.com/pocketonehq/pocketone/tree/main/ext/llm/examples/chat).

A chat page that answers questions about PocketOne and PocketBase, citing the docs it used and showing what each answer
cost.

```
pb_pages/
  index.ejs              the chat page
  chat/+api.ts           POST /chat: the answer as server-sent events; GET /chat: what the app knows
  chat/suggest/+api.ts   POST /chat/suggest: three follow-up questions
  _private/llm.ts        the model's name, the docs, search, the call to the chat
  _private/preamble.md   who the assistant is and how it answers
```

**What it knows.** `_private/llm.ts` loads two sets of docs in the background and refreshes them every ten minutes:
PocketOne's user docs from this site's `llms.txt` and `llms-full.txt`, and PocketBase's own docs from the Markdown copy
PocketOne keeps of pocketbase.io. It splits them into sections and indexes the words in each.

**Answering a question.**

1. `POST /chat` finds the three sections that match the question best (a BM25 search, with a few synonyms), each
   labelled with the docs it came from.
2. It sends the preamble, the conversation so far, and the question with those excerpts to the model, through a stored
   chat: `$llm.chat.open(id).sendAsync(question, { context, onText })`. The chat keeps the conversation, so the page
   holds only the chat's id.
3. Each piece of the answer is forwarded to the browser as it's written, with `ctx.res.sse()`, then a final event with
   the sources and the cost (`usage.costMicroUSD`, in millionths of a dollar).
4. A second call, `$llm.text.completeAsync({ schema })`, suggests three follow-up questions as JSON.

Every `$llm` call is awaited in an async handler, so the model works off the app's JS engine and the app keeps serving
other requests while an answer streams. Only the excerpts that match are sent, so each question is small and cheap, and
because the start of each prompt repeats, providers bill it at their cheaper cached rate.
