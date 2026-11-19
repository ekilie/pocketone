---
title: On a multisite host
description: "One model for every app on a server: each app gets a few calls at once and can switch to its own API."
---

# On a multisite host

A multisite host runs one model for every app it serves, instead of each app loading its own. Name it in the host's
`pocketone.yaml`:

```yaml
agent:
  model: qwen3-1.7b
  threads: 6
  parallel: 1             # answers written at once; the rest wait their turn
  per_instance:
    concurrency: 2        # calls an app may have waiting or running at once
    max_tokens: 512       # longest answer per call
```

Every app on the host then has `$agent` with no setup. An app that calls more often than its `concurrency` gets a
`busy` error straight away, so one busy app can't queue up the model for everyone, and no answer runs longer than
`max_tokens`.

The host decides which model runs, because it knows the machine: its memory, its cores and how many apps share them.
So an app's **Settings → LLM** offers three choices instead of a model list:

- **Your host's model** (the default): the model the host runs;
- **API**: the app's own OpenAI-compatible endpoint, for apps that need more than the shared model gives;
- **Off.**

Each app's prompts are cached separately on the host's model ([Prompt caching](prompt-caching.md#on-a-multisite-host)).

Today apps that run as isolates ([Pocker](../../pocker/README.md)) reach the host's model; apps in containers will
follow.
