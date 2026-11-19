---
title: Prompt caching
description: "The model keeps the start of recent prompts, so a repeated preamble or a chat's earlier turns aren't read again. How to order your prompts to get the most from it, and how to check it's working."
---

# Prompt caching

Before a model writes a word, it reads your whole prompt: the preamble, the excerpts, the conversation so far. On a
CPU, that reading is most of the wait. But most of a prompt is the same as last time: the same preamble on every call,
the same earlier turns in a chat.

**The built-in model remembers the start of recent prompts.** When your next prompt starts with exactly the same
text, it skips re-reading that part and starts from where the prompts differ. There's nothing to turn on, and the
answer is the same as if it had read everything again.

How much it helps, on a laptop, for a second prompt that shares a ~160-token preamble with the first:

| Model | Reading the prompt | With the cache |
|---|---|---|
| `qwen3-0.6b` | 0.47 s | 0.06 s |
| `qwen3-1.7b` | 2.2 s | 0.13 s |

## Order your prompt for it

The cache only matches an identical start. Once one token differs, everything after it is read again. So put what
never changes first and what changes most last:

1. **The preamble:** the same text on every call ([Writing a preamble](guides/preamble.md)).
2. **The conversation so far:** the earlier turns exactly as you sent them last time.
3. **The new question, then the excerpts** you picked for it ([Your own knowledge](guides/context.md)).

```ts
const r: AgentAnswer = $agent.complete({
  system: preamble, // never changes: read once, then reused
  messages: [
    ...earlierTurns, // same as last time: reused
    { role: "user", content: `${question}\n\nExcerpts to answer from:\n\n${excerpts}` }, // new: read
  ],
  maxTokens: 350,
})
```

```go
r, err := agentv1.Contract.Must(app).Complete(ctx, extkit.Internal, agentv1.CompleteInput{
	System: preamble, // never changes: read once, then reused
	Messages: append(earlierTurns, // same as last time: reused
		agentv1.Message{Role: "user", Content: question + "\n\nExcerpts to answer from:\n\n" + excerpts}), // new: read
	MaxTokens: 350,
})
```

The excerpts come after the question, in the same turn, because they change with every question. Anywhere earlier,
they'd change the start of every prompt after them. Put the question first, too: next time this turn is part of the
history and the excerpts are gone, so a prompt that started with the question still matches up to its end.

With this order, each turn of a chat reads only what's new since the last one: the previous answer, the new question
and its excerpts.

What breaks the cache:

- **Anything that changes at the top:** today's date, the user's name, a random example, a counter. Put it in the
  user's turn instead.
- **Rewriting history:** trimming or reformatting earlier turns changes the start. A rolling summary
  ([Remembering conversations](guides/conversations.md)) changes it each time it's rewritten, so rewrite it every few
  turns, not on every one.

## Check that it's working

Every answer reports it in `usage`:

```ts
const r: AgentAnswer = $agent.complete({ system: preamble, prompt: question })
// r.usage: { inputTokens: 431, outputTokens: 52, cachedTokens: 139 }
```

```go
r, err := agent.Complete(ctx, extkit.Internal, agentv1.CompleteInput{System: preamble, Prompt: question})
// r.Usage: {InputTokens: 431, OutputTokens: 52, CachedTokens: 139}
```

`cachedTokens` is how much of the prompt came from the cache. If it stays at 0 for prompts that should share a
preamble, something at the top differs between them: compare two prompts side by side.

## Settings

`pocketone.yaml`:

```yaml
agent:
  prompt_cache_mb: 256   # default; -1 turns it off
```

Each remembered token takes about 230 KB for `qwen3-0.6b` and `qwen3-1.7b` (290 KB for `qwen3-4b`), so the default
holds about 1,100 tokens. That's enough for a preamble and a short chat. The least recently used prompts make room for
new ones. On a small server, lower it to save memory; with more memory and long chats, raise it.

## Warm the preamble

The cache fills as prompts come in, so the first question after a start reads the whole preamble, and a burst of
other prompts can push it out. `$agent.warm` reads it ahead of time and keeps it:

```ts
// pb_pages/_private/preamble.ts: read once per app, warmed the first time a page asks for it
let text: string | undefined

export function preamble(ctx: any): string {
  if (text === undefined) {
    text = toString($os.readFile(`${ctx.pagesDir}/_private/preamble.md`))
    $agent.warm({ system: text }) // returns at once and warms in the background; never throws
  }
  return text
}
```

```go
// when the app serves, in the background (Warm returns at once without Wait)
app.OnServe().BindFunc(func(e *core.ServeEvent) error {
	agentv1.Contract.Must(app).Warm(context.Background(), extkit.Internal, agentv1.WarmInput{System: preamble})
	return e.Next()
})
```

It returns `{ id, warmed, message, forget() }`. `warmed` is false while it's still reading (or with your own API, which
caches by itself; `message` says why). In async page code, `await $agent.warm.async({ system: preamble })` resolves
once it's in. The id comes from the text, so warming the same text again finds the same warm and costs nothing; after
a model change it's warmed again on the new one. `forget()` lets it leave the cache like any other prompt.

A warmed prompt is pinned: it stays in `prompt_cache_mb` until forgotten, so warm the one or two preambles you use,
not every prompt. On a multisite host each app may pin up to `per_instance.pinned_mb`.

## Threads: a conversation's own state

The cache keeps the start of prompts. A chat needs more: the model's answer from last turn is part of the next prompt,
and reading it again costs as much as writing it did. `thread` keeps a conversation's whole state between calls, the
answer included:

```ts
const r: AgentAnswer = $agent.complete({ system: preamble, messages: turnsSoFar, thread: chatId })
```

```go
r, err := agent.Complete(ctx, extkit.Internal, agentv1.CompleteInput{System: preamble, Messages: turnsSoFar, Thread: chatID})
```

The next call with the same `thread` continues from that state as far as its messages start the same way, so it reads
only what's new. For that, send the earlier turns exactly as you sent them, question text and excerpts included,
followed by the model's answer exactly as it came back. Trimming or rewriting the history costs one full read, then
the thread carries on from there: trim rarely and in big steps, not one turn at a time (`$agent.count(messages)`
tells you when the conversation passes your budget).

Thread states live in memory (`thread_memory_mb`, default 512) and on disk, at half precision, in
`pb_cache/agent/threads` (`thread_cache_gb`, default 2): about 115 KB a token on disk for `qwen3-0.6b` and `qwen3-1.7b`.
The least recently used go first. Reading one back from disk takes a fraction of a second; reading a 2,000-token chat
again takes 15 to 60 seconds on a small CPU. `$agent.threads.list()` shows what's kept, `$agent.threads.forget(id)`
drops one. Like the rest of `pb_cache`, they're never backed up and safe to delete.

[Agent Chat](../../agent-chat/README.md) uses the chat's id as its thread and keeps the start stable for you.

## On a multisite host

The host's model keeps each app's prompts apart: an app only reuses the start of its own prompts, never another app's,
so answer timing can't reveal what another app sent. The memory budget is the host's, shared by all of them. Each
app's warms are capped by `per_instance.pinned_mb`, and its threads are kept in its own `pb_cache` (counting against
its disk) within `per_instance.thread_cache_gb`.

## With your own API

Big providers cache prompts too, so the same ordering makes your own API faster and cheaper:

- **OpenAI** caches by itself for prompts over about a thousand tokens.
- **Anthropic** caches up to the points you mark in the request (`cache_control`).
- **Google Gemini** caches automatically, and can also hold a named cache with a lifetime for a large shared document.

When the provider reports how much it reused (OpenAI does), it shows up as `usage.cachedTokens` too.

A cache isn't memory. It makes text you send again faster to read, but you still send it every time: what goes into
the prompt is still yours to decide ([Remembering conversations](guides/conversations.md)).
