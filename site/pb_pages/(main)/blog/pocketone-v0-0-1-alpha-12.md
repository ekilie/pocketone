---
title: "PocketOne alpha.12: JS hooks stop crashing on shared maps"
description: PocketOne v0.0.1-alpha.12 fixes a crash where a JS hook could take down the whole app with "concurrent map iteration and map write".
date: 2026-11-21
author: Ben Allfree
tags: release
---

Last night the PocketHost mothership went down with this:

```
fatal error: concurrent map iteration and map write
```

That's not a panic you can recover from. Go kills the whole process.

The cause was in JSVM's memory limit. Every time a hook calls into Go, PocketOne measures what came back so a runaway script can't eat the server. It measured by walking everything reachable from the result, maps included. Some of those maps belong to other goroutines and are written under their own locks. Walk one while it's being written and the process dies.

Alpha.12 measures smarter. Types that know their own size (records, for example) report it under their own locks. Structs that hold a mutex get charged their size without being walked. Maps behind a pointer get charged from their length instead of being iterated. The numbers stay close to the real thing, and a regression test reproduces the crash before the fix and passes under `-race` after.

If you run JS hooks with any real traffic, upgrade: `brew upgrade pocketone` or `pocketone upgrade`.

Also in this release, every dashboard on the standard set shows PocketOne's logo and version, and its Docs link goes to pocketone.dev. That includes instances on multisite hosts, not just the `pocketone` binary.

[Release notes](https://github.com/pocketonehq/pocketone/releases/tag/v0.0.1-alpha.12)
