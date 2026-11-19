# Agent changelog

Changes to `ext/agent`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

## 0.0.2 (2026-11-18)

### Added

- Threads: `thread: "<id>"` on a call keeps the conversation's state (what the built-in model read and wrote) in memory (`agent.thread_memory_mb`, 512 MB by default) and on disk in `pb_cache/agent/threads` (`agent.thread_cache_gb`, 2 GB by default), so the next turn reads only what's new, also after a restart. `$agent.threads.list()` and `$agent.threads.forget(id)`. On a multisite host each app's threads are in its own `pb_cache`, within `per_instance.thread_cache_gb` (2.5 GB by default).
- `$agent.warm({ system })` reads a preamble into the prompt cache ahead of the calls that use it and keeps it there until `forget()`; it returns `{ id, warmed, forget() }` at once, and `await $agent.warm.async(...)` once it's warm. On a multisite host each app's warms are capped by `per_instance.pinned_mb` (64 MB by default).
- `$agent.count(textOrMessages)`: tokens for the app's model (an estimate with your own API).
- Waiting for the model: `$agent.complete` fails at once with `not_ready` while the model loads, and `await $agent.complete.async(opts)` waits for it (up to `opts.wait`, default `agent.timeout`); `$agent.stream` takes `wait` and `{ onWait }` to show progress. Over HTTP, agent/v1 takes `wait`.
- `$agent.newChat()`, `$agent.chat(id)` and `$agent.chats`, when an extension provides agent.chat/v1 ([Agent Chat](../agent-chat/README.md)).
- agent/v1 operations `warm`, `forget`, `count`, `threads` and `threads/forget`, and `thread` and `wait` on calls.

### Changed

- Earlier answers in a conversation are sent to the built-in model with the empty thinking block it wrote them with, so they read exactly as written and a thread covers them.

- Prompt caching in the built-in model: it keeps the start of recent prompts (`agent.prompt_cache_mb`, 256 MB by default; `-1` turns it off), so a prompt that begins the same way, such as the same preamble or a chat so far, isn't read again; a second prompt sharing a preamble starts answering several times sooner. `usage.cachedTokens` says how much of a prompt came from a cache, also for APIs that report it. On a multisite host each app's prompts are cached apart from the others'.
- `$agent.stream(opts, onText)`: the answer a piece at a time as the model writes it, from the built-in model or your own API (asked to stream), and on a multisite host from its shared model; return `false` from `onText` to stop. Over HTTP it's agent/v1's `stream` operation (`POST /api/_c/agent/v1/stream`, server-sent events: a chunk per piece, then one with the result).
- New extension: a language model for every app, as agent/v1 and `$agent.complete()` in `pb_hooks` (with an optional JSON schema; the parsed answer comes back as `data`, and a model PocketOne runs can only write JSON that matches it). Settings → LLM in the dashboard picks what answers: off, a built-in model (a list; switching downloads the new one and unloads the old) or your own OpenAI-compatible API; until then `pocketone.yaml` decides, with a model PocketOne runs itself (`pocketone.yaml`'s `agent.model`: `qwen3-0.6b`, `qwen3-1.7b`, `qwen3-4b`, an `hf:` file or a local `.gguf`; downloaded once, checked and cached), in pure Go with no cgo. On a multisite host whose `agent` section names a model, every Pocker instance shares it, with a per-instance limit on calls at once and on answer length, and each instance's Settings → LLM offers off, the host's model or its own API (the host decides which model runs built in).

