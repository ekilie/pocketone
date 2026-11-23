---
title: "PocketOne alpha.13: cloud models in your app"
description: PocketOne v0.0.1-alpha.13 replaces Agent with LLM, cloud models on your own keys, and ships two live example apps.
date: 2026-11-22
author: Ben Allfree
tags: release
---

This one is all about AI. Agent is gone and LLM takes its place.

**LLM replaces Agent.** Agent ran one built-in model. LLM talks to the clouds instead, on your own keys: Vultr, Cloudflare Workers AI, OpenRouter, or anything that speaks the OpenAI API. Add a provider in Settings → LLM, hit Test, and call it from your pages:

```js
const r = await $llm.text.completeAsync({
  model: 'vultr/glm-5.3',
  prompt: 'Summarize in one sentence: ' + record.getString('body'),
})
// r.text is the summary
```

It's not just text. `$llm` draws pictures, reads text aloud, writes down what's said in audio, makes music and video, and turns text into embeddings. Every call runs off the JS VM, so your app keeps serving while a model works. And every result tells you what it cost. More in [AI in your app](/docs/extensions/llm).

**An example app and a new starter kit.** [Bedtime Story](https://sleep.pocketone.dev) picks one topic and tells it slowly, read aloud with pictures and music, and the new [Example apps](/docs/examples) section walks through how it's built. The new `ask` [starter kit](/docs/starters/ask) answers questions from your own docs: `pocketone pages create mydocs --starter ask`. Both are plain PocketOne apps, no Go.

**Async handlers can wait longer.** A route, hook or cron job can now await for up to 15 minutes in all. Before, anything waiting more than 30 seconds timed out, which was no good for a model making a video. If the client goes away, the handler stops.

**A few fixes.** Superuser UI extensions registered from `pb_hooks` now work, including inside Pocker instances. Dev mode stopped logging every hook file with `<nil>`. And on Homebrew, `pocketone upgrade` now tells you `brew update && brew upgrade pocketone`, so you don't miss a release that's only hours old.

To upgrade, run `pocketone upgrade`. If you used `$agent`, add a provider in Settings → LLM and switch to `await $llm.text.completeAsync({ model: 'provider/model', ... })`. The [release notes](https://github.com/pocketonehq/pocketone/releases/tag/v0.0.1-alpha.13) have the full steps, Agent Chat included.
