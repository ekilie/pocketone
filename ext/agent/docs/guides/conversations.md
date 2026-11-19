---
title: Remembering conversations
description: "A model remembers nothing between calls: keep chats in collections, send the recent turns, summarize the older ones, and recall what matters from past chats."
---

# Remembering conversations

A model remembers nothing between calls. A chat feels continuous only because the app sends the conversation again
with every question. So "memory" is something you build: you decide what to keep, where, and how much of it to send
back each time.

## Keep chats in your app

Store every chat as records, so it survives reloads and belongs to the person who had it. Two collections are enough:

| Collection | Fields | API rules |
|---|---|---|
| `chats` | `user` (relation to users), `title` (text), `summary` (text) | `user = @request.auth.id` |
| `messages` | `chat` (relation to chats), `role` (select: user, assistant), `content` (text) | `chat.user = @request.auth.id` |

Each question then loads what it needs, asks, and saves both turns:

```js
// pb_pages/_private/chat.js
const preamble = (ctx) => toString($os.readFile(`${ctx.pagesDir}/_private/preamble.md`))

function ask(ctx, chatId, question) {
  const app = ctx.app
  const chat = app.findRecordById("chats", chatId)
  if (chat.getString("user") !== ctx.auth?.id) throw ctx.httpError(404, "no such chat")

  const recent = app
    .findRecordsByFilter("messages", "chat = {:chat}", "-created", 6, 0, { chat: chatId })
    .reverse()
    .map((m) => ({ role: m.getString("role"), content: m.getString("content") }))

  const summary = chat.getString("summary")
  const system = preamble(ctx) + (summary ? `\n\nEarlier in this conversation: ${summary}` : "")
  const r = $agent.complete({ system, messages: [...recent, { role: "user", content: question }], maxTokens: 350 })

  const save = (role, content) => {
    const m = new Record(app.findCollectionByNameOrId("messages"))
    m.set("chat", chatId)
    m.set("role", role)
    m.set("content", content)
    app.save(m)
  }
  save("user", question)
  save("assistant", r.text)
  return r.text
}
```

## Send the recent turns, summarize the rest

Sending the whole chat every time works for a while, then gets slow (a built-in model reads every word, every call)
and finally doesn't fit. The usual answer has two parts:

- **A window:** send only the last few turns as messages (six above).
- **A summary of everything before it:** when a chat grows past the window, ask the model to fold the turns that drop
  out into a few sentences, and keep that in the chat's `summary`:

```js
function summarize(ctx, chat, dropping) {
  const r = $agent.complete({
    system: "Update the summary of this conversation. Keep names, numbers, decisions and open questions. At most 80 words.",
    prompt: `Summary so far: ${chat.getString("summary") || "(none)"}\n\nNew turns:\n` +
      dropping.map((m) => `${m.role}: ${m.content}`).join("\n"),
    maxTokens: 150,
    temperature: 0.2,
  })
  chat.set("summary", r.text.trim())
  ctx.app.save(chat)
}
```

Run it when a chat passes the window, or every few turns, not on every question: it's an extra call. A
[cron](https://pocketbase.io/docs/js-jobs-scheduling/) can do it for idle chats, off the request path.

## Remember across chats

What someone said last week ("I'm on the Team plan", "we deploy with GitHub Actions") can carry into a new chat the
same way you add your docs as context:

- keep short **notes** per user in a collection: a fact per record, written when the user states one, or pulled out
  at the end of a chat with a [schema](structured.md) (`{ type: "array", items: { type: "string" }, maxItems: 5 }`);
- with each new question, find the notes that match it (the same kind of search as in
  [Your own knowledge](context.md)) and add them under the preamble: "What you know about this user: ...".

Let people see and delete their notes. Only use a user's notes in that user's chats.

## What big models cache

Big providers make long, repeated prompts cheaper with **prompt caching**. While a model reads a prompt, it computes
an internal state for every token, and the provider keeps that state for the start of a prompt for a few minutes. When
your next call starts with exactly the same text, the model skips re-reading it, so it costs less and answers sooner:

- **OpenAI** does it by itself for prompts over about a thousand tokens.
- **Anthropic** caches up to the points you mark in the request (`cache_control`).
- **Google Gemini** caches automatically, and also lets you create a named cache with a lifetime for a large shared
  document.

The cache matches the beginning of the prompt only, and only if it's identical. That's why the order is: the
preamble (never changes), then your docs or notes (change per question), then the conversation (grows), then the new
question. A date or name at the top invalidates everything after it. A cache isn't memory, either: it's a speed-up for
text you send again, not a way to stop sending it.

A model PocketOne runs reads the whole prompt on every call; it keeps nothing between calls. That makes the window and
the summary above matter more: on a small server, a few hundred words of preamble and context and the last few turns
is about right.
