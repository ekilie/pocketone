---
title: "PocketOne alpha.11: install anywhere, clone any app"
description: PocketOne v0.0.1-alpha.11 adds new installation tools and pocketone clone.
date: 2026-11-21
author: Ben Allfree
tags: release
---

Two things this time. It's easier to get PocketOne onto your machine, and easier to get your app back off the server.

**New installation tools.** On Windows, `scoop install pocketone` (after `scoop bucket add pocketonehq https://github.com/pocketonehq/scoop-bucket`). On macOS or Linux, `curl -fsSL https://pocketone.dev/install.sh | sh`. Running a Linux server? Grab the `.deb` or `.rpm` and you get a systemd service out of the box. It's all on the [Installation](/docs/installation) page.

**`pocketone clone`.** Got an app running somewhere? `pocketone clone https://myapp.example.com` gives you a local copy, signed in and ready. Then `pocketone dev` and you're editing live.

**One fix.** `pocketone deploy` no longer overwrites files somebody changed on the server since you last synced. It tells you which ones and stops. Add `--force` if you really mean it.

To upgrade, run `pocketone upgrade`. On Homebrew, the formula moved: `brew uninstall pocketone`, then `brew install pocketonehq/tap/pocketone`. Multisite hosts and anyone passing `--commit` or `-m` to deploy, check the [release notes](https://github.com/pocketonehq/pocketone/releases/tag/v0.0.1-alpha.11) first.
