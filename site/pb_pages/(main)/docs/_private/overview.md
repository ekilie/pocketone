---
title: What is PocketOne?
description: PocketOne is a community fork of PocketBase. Its core changes are kept small; everything else is a plugin.
---

# What is PocketOne?

PocketOne is a community fork of [PocketBase](https://pocketbase.io). It tracks upstream PocketBase and keeps its own
changes to the core small: only the hooks that let plugins extend and override PocketBase. Every feature lives in a
plugin.

It's maintained by [PocketHost](https://pockethost.io), which runs it in production. PocketOne is not affiliated with
or endorsed by the PocketBase project.

## How it's built

- **A small core.** PocketBase's own code changes only to add hooks. Each change is marked in the source, reviewed by
  someone who didn't write it, and kept minimal so upstream releases merge cleanly.
- **Everything else is a plugin.** Plugins live in `ext/` in the repo. Each is its own Go module, with a README and a
  small example.
- **Off until you use it.** The plugins in the `pocketone` binary add no work to an app that doesn't use them.
- **One app or hundreds.** A plugin written against the plugin contract works the same on a single app and on every
  instance under a multi-instance host.

## Open to contributions

PocketOne is open to everyone. Ask questions and share ideas in
[GitHub Discussions](https://github.com/pockethost/pocketone/discussions), report bugs in
[Issues](https://github.com/pockethost/pocketone/issues), and send plugins as pull requests. Start with
[Writing a plugin](https://github.com/pockethost/pocketone/blob/main/docs/plugins.md).

## Where to next

- [Install PocketOne](/docs/installation)
- [See what PocketOne adds to PocketBase](/docs/what-pocketone-adds)
- [Browse the plugins](/docs/plugins)
