---
title: PocketBase compatibility
description: PocketOne's core runs PocketBase's own test suite, unmodified, on every change and every day. What runs, the one known exception, and the latest result.
---

# PocketBase compatibility

[![PocketBase tests](https://github.com/pockethost/pocketone/actions/workflows/upstream-tests.yml/badge.svg?branch=main)](https://github.com/pockethost/pocketone/actions/workflows/upstream-tests.yml?query=branch%3Amain)

PocketOne changes PocketBase's own code only to add hooks for extensions and to fix a few bugs. Those changes must
leave PocketBase working as PocketBase does. We check that by running PocketBase's own test suite, exactly as PocketBase
wrote it, against PocketOne's core.

The badge above is the latest run on `main`. Click it for every run and its results: the PocketBase commit the tests
came from, and how many passed, failed and were skipped.

## What runs

The [`PocketBase tests`](https://github.com/pockethost/pocketone/blob/main/.github/workflows/upstream-tests.yml)
workflow runs on every change to PocketOne's core and once a day. It has two jobs, and the badge is green only when
both pass:

- **Upstream's tests, unmodified.** Every test file is put back to the version in the PocketBase release PocketOne is
  built on. Test files we changed are restored and test files we added are removed. Then the whole suite runs: more than
  5,000 tests across 35 packages.
- **Core tests, ours included.** The same suite as it is in PocketOne, with our tests for each hook and bug fix.

A few upstream tests wait a fixed time for events and can miss it when the whole suite runs at once on a busy
machine. A test that fails gets two more tries on its own. If it passes on one, the run still passes, and the run's
summary lists the test as flaky.

Extensions aren't part of either job. They have their own tests, which run separately.

## Known exceptions

One upstream test checks something PocketOne changes on purpose, so it's skipped:

| Test | Why it's skipped |
|---|---|
| `TestHooksBindsCount` (`plugins/jsvm`) | It counts the hooks JavaScript can bind to. PocketOne adds one, `onFilesystemCreate`, so there are 84 instead of 83. |

The list lives in
[`known-deviations.txt`](https://github.com/pockethost/pocketone/blob/main/.github/upstream-tests/known-deviations.txt).
Anything not on it must pass.

## Run it yourself

In a PocketOne clone, add PocketHost's mirror of PocketBase as `upstream`, without its tags:

```bash
git remote add upstream https://github.com/pockethost/pocketbase.git
git config remote.upstream.tagopt --no-tags
git fetch upstream master
```

Then, on a checkout you don't mind changing (the script rewrites test files):

```bash
.github/upstream-tests/run.sh
```

## What changed in core

Every core change is listed, with a diff for each, in the README's
[Changes we made to PocketBase](https://github.com/pockethost/pocketone#changes-we-made-to-pocketbase).
