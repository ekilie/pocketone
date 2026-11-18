---
title: Coming from PocketBase
description: PocketOne is PocketBase with more built in. What stays the same, what's added, and where to read about the parts that come straight from PocketBase.
---

# Coming from PocketBase

PocketOne is PocketBase with more built in. Run `pocketone` the way you run `pocketbase`: the same commands and flags,
the same API, the same dashboard and the same `pb_data`, `pb_hooks` and `pb_migrations` folders.

## What stays the same

- **Your API and SDKs.** Collections, records, auth, files and realtime work as in PocketBase. The JS and Dart SDKs
  work unchanged.
- **Your hooks.** The JS API (`$app`, `routerAdd`, `onRecordCreate`, `cronAdd`, …) is PocketBase's, so existing
  `pb_hooks` run as they are. They run on [QuickJS](/docs/quickjs), which adds imports, TypeScript and async handlers on
  top.
- **PocketBase's own tests.** PocketBase's test suite runs unmodified against PocketOne's core on every change and
  every day. See the live results on [PocketBase compatibility](/docs/compatibility).

PocketOne is built on PocketBase v0.42. When pocketbase.io describes a newer release, some of it may not be in
PocketOne yet.

## What's added

Everything below is on by default. Each row shows how PocketBase handles it and what PocketOne adds; the links go to
the feature's page.

| | In PocketBase | In PocketOne |
|---|---|---|
| [QuickJS hooks](/docs/quickjs) | Hooks on goja: CommonJS `require`, synchronous handlers, a pool of runtimes | `async`/`await`, ES module `import`, TypeScript, shared globals and hard limits, on the same JS API |
| [PocketPages sites](/docs/features/sites) | Static files and single-page apps from `pb_public`; HTML from hooks with `$template` | Server-rendered pages in TSX, JSX, TypeScript, EJS or Markdown, with `await` everywhere and no build step |
| [PocketBase isolates](/docs/features/isolates) | One app per process, run side by side in processes, containers or VMs | Hundreds of apps in one process, each starting in tens of ms, about 3 MB each, sandboxed |
| [Point-in-time restore](/docs/features/restore) | Backups you schedule, restored whole | Every change recorded; restore to any second in the last 72 hours, and undo the restore |
| [Deploy and live sync](/docs/features/deploy) | Copy the app folder to the server; hooks reload, migrations run on the next start | `pocketone deploy` uploads only what changed; `pocketone dev` syncs a folder both ways and streams logs |
| [Remote CLI](/docs/features/remote) | A CLI on the machine that holds `pb_data`; the dashboard and REST API from anywhere | Admin commands against any app, from your terminal, a script or an agent |
| [API tokens and OAuth](/docs/features/tokens) | Auth tokens that act as their account | Named, scoped tokens you revoke one by one; your app is an OAuth server |
| [MCP for AI agents](/docs/features/mcp) | A REST API and realtime, with JavaScript and Dart SDKs | `/mcp` on every app, with OAuth sign-in and your own tools from `pb_hooks` |
| [Bulk mail](/docs/features/bulk-mail) | Account mail (verification, password reset), and any mail from hooks | Campaigns from the dashboard, with unsubscribes, suppression and pacing |
| [pocketone.yaml](/docs/pocketone-yaml) | Settings in the dashboard; `serve` options as flags | One config file, with every extension's options and a `.env` for secrets |

Smaller additions are listed under [Also built in](/docs/features#also-built-in).

## Read the PocketBase docs for the rest

This site doesn't copy PocketBase's docs. For everything PocketOne leaves as it is, read them at pocketbase.io:

- [Introduction](https://pocketbase.io/docs/)
- [Collections](https://pocketbase.io/docs/collections/)
- [API rules and filters](https://pocketbase.io/docs/api-rules-and-filters/)
- [Authentication](https://pocketbase.io/docs/authentication/)
- [Files upload and handling](https://pocketbase.io/docs/files-handling/)
- [Working with relations](https://pocketbase.io/docs/working-with-relations/)
- [Extend with JavaScript](https://pocketbase.io/docs/js-overview/)
- [Extend with Go](https://pocketbase.io/docs/go-overview/)
- [Going to production](https://pocketbase.io/docs/going-to-production/)
- [Web APIs reference](https://pocketbase.io/docs/api-records/)
