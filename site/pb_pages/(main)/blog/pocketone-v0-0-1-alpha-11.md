---
title: "PocketOne alpha.11: install anywhere, clone any app"
description: PocketOne v0.0.1-alpha.11 adds Scoop, an install script and Linux packages, pocketone clone and safer deploys.
date: 2026-11-21
author: Ben Allfree
tags: release
---

Alpha.11 is about getting PocketOne onto your machine and getting your app back off the server.

**New installation tools.** Windows gets Scoop: `scoop bucket add pocketonehq https://github.com/pocketonehq/scoop-bucket`, then `scoop install pocketone`. On macOS and Linux without Homebrew, `curl -fsSL https://pocketone.dev/install.sh | sh` installs the newest release and checks its checksum. Run it again to upgrade. Every release also ships `.deb`, `.rpm` and `.apk` packages. The `.deb` and `.rpm` set up a `pocketone` systemd service with its own user, config in `/etc/pocketone` and data in `/var/lib/pocketone`. It's all on one [Installation](/docs/installation) page now.

**`pocketone clone`.** Point it at a running app and you get a local project: `pocketone clone https://myapp.example.com` makes the folder, signs you in as a superuser in the browser, adds the app as the default remote and pulls its code. Then `pocketone dev` and you're editing live.

**Deploys don't clobber the server.** If someone edited a file on the server, or the dashboard wrote a migration, since your last deploy, pull or dev, `pocketone deploy` lists those files and stops. `--force` overwrites them. Deploys are no longer recorded, so `--commit` and `-m` are gone. Pull and clone now download in bundles, and the CLI waits and retries when a server rate limits it.

## Upgrading

- Homebrew moved taps: `brew uninstall pocketone`, then `brew install pocketonehq/tap/pocketone`.
- Everyone else: `pocketone upgrade`.
- Running a multisite host? Check the upgrade steps in the [release notes](https://github.com/pocketonehq/pocketone/releases/tag/v0.0.1-alpha.11) for the deploy extension and multisite hosts.

[Release notes](https://github.com/pocketonehq/pocketone/releases/tag/v0.0.1-alpha.11)
