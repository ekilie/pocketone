---
title: On a multisite host
description: "One model for every app on a server: each app gets a fair share and a daily allowance, and can switch to its own API."
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
    tokens_per_day: 50000 # answer tokens an app may use a day (0: no limit)
    max_tokens: 512       # longest answer per call
```

Every app on the host then has `$agent` with no setup. An app that calls more often than its `concurrency` gets a
`busy` error straight away, and one past its daily allowance gets `quota` until midnight UTC, so one busy app can't
crowd out the rest.

The host decides which model runs, because it knows the machine: its memory, its cores and how many apps share them.
So an app's **Settings → LLM** offers three choices instead of a model list:

- **Your host's model** (the default): the host's model, with the app's use today against its allowance;
- **API**: the app's own OpenAI-compatible endpoint, for apps that need more than the shared model gives;
- **Off.**

Today apps that run as isolates ([Pocker](../../pocker/README.md)) reach the host's model; apps in containers will
follow.
