---
title: "PocketOne alpha.8: a language model, WASM modules and desktop apps"
description: PocketOne v0.0.1-alpha.8 gives every app a language model ($agent), runs WASM modules from pb_hooks, and builds your app into a native desktop app. It's on PocketBase v0.42.3.
date: 2026-11-19
author: Ben Allfree
tags: release
---

This is the biggest PocketOne release so far. Three new things ship in the `pocketone` binary: a language model your hooks can call, WASM modules you can drop into `pb_hooks`, and a command that turns your app into a desktop app.

## A language model in every app

Most apps have small jobs that want a model. Label a support ticket. Pull a JSON object out of an email. Write a title or an alt text. Until now that meant wiring up an API client in your hooks.

Now it's `$agent.complete()`:

```js
const { data } = $agent.complete({
  prompt: `Classify this ticket: ${record.getString('body')}`,
  schema: { type: 'object', properties: { label: { enum: ['bug', 'billing', 'question'] } } },
})
```

The answer can come from your own OpenAI-compatible API (OpenAI, OpenRouter, Groq, Ollama) or from a small model PocketOne runs itself, on your server's CPU, in the same binary. No Python, no GPU, nothing else to install. You pick which one in the dashboard under **Settings → LLM**, and your code doesn't change between them.

Give it a JSON schema and the answer comes back parsed, and a built-in model can only write JSON that matches. `$agent.stream()` gives you the answer a piece at a time. Threads and prompt caching keep follow-up turns fast, and [Agent Chat](/docs/extensions/agent-chat) stores conversations with their history.

It's off until you choose a model, so nothing downloads or runs unless you ask for it. Start with the [Agent docs](/docs/extensions/agent).

## WASM modules

Some work is better in another language. A Markdown renderer, an image thumbnailer, a QR code generator: there's a good library for each in Go, Rust or Zig.

Build it as a WASI module, put it in `pb_hooks` as `<name>.pb.wasm`, and call it:

```js
const html = await $wasm.run.async('markdown', { text: record.getString('body') })
```

Each call runs in its own box, with its own memory and time limits, off the app's JS VM. A module can't reach your files, your database or the network. Input goes in, output comes out.

The [WASM docs](/docs/extensions/wasm) have a page for each language (C, Go, TinyGo, Rust, Zig and others) with what each one costs per call, plus recipes built on real libraries. Thanks to @octavonu, who built it.

## Desktop apps

The app folder you run with `pocketone serve` can now be a desktop app:

```bash
pocketone desktop build --all
```

That compiles your `pb_hooks`, `pb_migrations` and `pb_public` into one native executable each for macOS, Windows and Linux, from whichever machine you're on. The app runs its server on a private port and shows it in a native window. Builds run in Docker, in a build image pinned to your `pocketone` version. See [Desktop](/docs/extensions/desktop).

## Also in this release

- **PocketBase v0.42.3.** Realtime and OAuth2 redirect fixes from upstream.
- **Extensions can own their schema.** An extension's migrations run at startup, kept apart from your app's `pb_migrations`.
- **`types.d.ts` covers every contract**, generated from its Go interface, and now sits in your project root.
- **`pb_cache/`** next to `pb_data` holds files extensions can rebuild. Add it to your `.gitignore`.

## Before you upgrade

Two things change under you:

1. The Tokens extension is now **API Tokens**. Rename `tokens:` to `api-tokens:` in `pocketone.yaml` and use `$apiTokens` in your hooks (was `$tokens`). Its collections are renamed on the first start.
2. Go import paths moved to `pocketone.dev`. If you build PocketOne into your own Go program, replace `github.com/pockethost/pocketone/` with `pocketone.dev/` in your imports. The repo itself now lives at [github.com/pocketonehq/pocketone](https://github.com/pocketonehq/pocketone).

Upgrade with `brew upgrade pocketone` or `pocketone upgrade`. Everything else is in the [release notes](https://github.com/pocketonehq/pocketone/releases/tag/v0.0.1-alpha.8).
