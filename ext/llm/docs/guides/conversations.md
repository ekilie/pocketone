---
title: Conversations
description: "Chats that remember: send the conversation with each call, or let LLM Chat store it, summarize it and send it for you."
---

# Conversations

Cloud models remember nothing between calls: every call sends the whole conversation. For a short exchange, send it
yourself as `messages`:

```ts
const r = await $llm.text.completeAsync({
  model: "vultr/glm-5.2",
  system: "You help with our notes app. Be brief.",
  messages: [
    { role: "user", content: "How do I export my notes?" },
    { role: "assistant", content: "Settings → Export, then pick Markdown or PDF." },
  ],
  prompt: "And can I export just one folder?", // added as the last user message
})
```

For chats that last, the [LLM Chat](../../../llm-chat/README.md) extension stores them for you: it keeps every turn,
sends the recent ones with each question, and folds older ones into a summary so long chats stay quick and cheap.

```ts
const id: string = $llm.chat.new({ meta: { user: ctx.auth.id } })
const chat = $llm.chat.open(id, { model: "vultr/glm-5.2", system: "You help with our notes app. Be brief." })
const answer: LlmTextAnswer = await chat.sendAsync("How do I export my notes?")
```

Keep the id on your own record (or hand it to the browser) and open the chat again on the next question. `history()`
reads its turns, `forget()` deletes it, and `$llm.chat.list({ meta })` finds a user's chats. With `onText`, `send`
streams the answer as it's written.

## Keep the start the same

Providers cache the start of recent prompts and bill it cheaper when the next prompt starts the same way. LLM Chat
keeps the start of each turn identical until the next summary: the same system text, the same summary, the same earlier
turns. Do the same in your own calls: put what changes (the question, the passages that answer it) at the end, and keep
the system text the same from call to call.
