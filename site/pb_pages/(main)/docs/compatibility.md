---
title: PocketBase compatibility
description: Live results of PocketBase's own test suite, unmodified, run against PocketOne's core on every core change and daily.
---

# PocketBase compatibility

PocketBase's own test suite, exactly as PocketBase wrote it, runs against PocketOne's core on every core change and
every day.

<%- include('compat-live.ejs') %>

## Known exceptions

Tests skipped because PocketOne changes what they check on purpose. Everything else must pass.

| Test | Package | Why |
|---|---|---|
| `TestHooksBindsCount` | `plugins/jsvm` | Counts JS hook binds: 84 with our `onFilesystemCreate`, 83 upstream |

## What runs

| Job | Test files | Skipped | A failed test |
|---|---|---|---|
| upstream's tests, unmodified | PocketBase's, from the release PocketOne is built on; ours removed | [known exceptions](#known-exceptions) | gets 2 more tries alone; a pass there is listed as flaky |
| core tests, ours included | as on `main` | none | same |

| Runs on | Covers | Doesn't cover |
|---|---|---|
| a core change on `main`, pull requests, daily at 14:00 UTC | the core Go module: 35 packages | extensions (tested on their own) |

## Run it yourself

On a scratch checkout (the script rewrites test files):

```bash
git remote add upstream https://github.com/pockethost/pocketbase.git
git config remote.upstream.tagopt --no-tags
git fetch upstream master
.github/upstream-tests/run.sh
```

| Source | |
|---|---|
| Workflow | [`upstream-tests.yml`](https://github.com/pockethost/pocketone/blob/main/.github/workflows/upstream-tests.yml) |
| Script | [`run.sh`](https://github.com/pockethost/pocketone/blob/main/.github/upstream-tests/run.sh) |
| Exceptions | [`known-deviations.txt`](https://github.com/pockethost/pocketone/blob/main/.github/upstream-tests/known-deviations.txt) |
| Every core change | [Changes we made to PocketBase](https://github.com/pockethost/pocketone#changes-we-made-to-pocketbase) |
