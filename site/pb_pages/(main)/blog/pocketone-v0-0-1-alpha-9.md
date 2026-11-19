---
title: "PocketOne alpha.9: a fix for Pocker hosts"
description: PocketOne v0.0.1-alpha.9 fixes Pocker instances that failed to start after upgrading to alpha.8, and collection renames on Pocker instances.
date: 2026-11-19
author: Ben Allfree
tags: release
---

Alpha.8 shipped with a bug that only shows up on multisite hosts running Pocker. I found it the hard way, rolling it out to PocketHost's edges this morning.

On upgrade, alpha.8 renames the API Tokens tables. On a Pocker instance that rename failed with `near "RENAME": syntax error`, and the instance wouldn't start.

The cause was older than alpha.8. Pocker opens every instance's database through the sandbox's SQL guard, under its own driver name. The database layer picks its SQL dialect by driver name, didn't know that one, and fell back to generic SQL. So renaming any collection on a Pocker instance had been broken all along. The API Tokens rename was just the first one to run at startup.

Alpha.9 teaches it the guard's driver is SQLite, with a test that reproduces the exact failure.

If you run `pocketone serve` for a single app, nothing changes for you: it never used the guard. If you run a multisite host on alpha.8, upgrade with `brew upgrade pocketone` or `pocketone upgrade`.

[Release notes](https://github.com/pocketonehq/pocketone/releases/tag/v0.0.1-alpha.9)
