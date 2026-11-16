---
title: What PocketOne adds
description: What changes when you run pocketone instead of pocketbase, and what's new.
---

# What PocketOne adds

Run `pocketone` the way you run `pocketbase`. These are the differences you'll notice.

## Point-in-time restore, on by default

Your database is replicated with Litestream, so you can restore it to any moment from **Settings → Backups** or with
`pocketone pitr restore`. Turn it off with `--pitr=false`.

## Mail you can see in dev mode

In dev mode (`serve --dev`), an app with no SMTP settings doesn't send through the system's `sendmail`. The mail is
caught and shown at `http://127.0.0.1:8025`.

## Server-rendered pages

Put `.ejs` and Markdown files in a `pb_pages` folder and they're served as pages, with no build step. This site is
one. `pocketone pages create myapp` starts a new site from a starter kit. Full docs are at
[pocketpages.dev](https://pocketpages.dev).

## A config file

Settings can live in `pocketone.yaml`, with secrets in a `.env` file beside it that your JS hooks never see. Without
the file, PocketOne behaves like stock PocketBase. `pocketone config` lists every section, and
`pocketone config check` checks a file. The [config reference](/docs/config-reference) lists every section.

## Commands for working with remote apps

- `pocketone login`, `remote` and `link`: save logins to your apps.
- `pocketone deploy`, `dev`, `pull` and `logs`: ship `pb_hooks`, `pb_migrations` and `pb_public` to an app and
  watch it.
- `pocketone upgrade`: move to the latest release, keeping the old executable.

## Agents and API tokens

- An MCP endpoint at `/mcp` lets agents such as Claude manage your app and call its tools.
- Long-lived API tokens (`pbt_…`) with scopes, for scripts and integrations.
- Your app can act as an OAuth provider; `pocketone login` and MCP sign-in use it.

## Extensions that fit together

PocketOne's extensions meet through contracts: named, versioned promises like `restore/v1` that any extension can
keep. The dashboard, the `pocketone` command, agents and your `pb_hooks` call the contract, so you can swap the
extension behind it without changing anything else. [How PocketOne fits together](/docs/contracts) explains it.

## Everything else

Automatic TLS certificates, bulk mail to your own users, scheduled VACUUM, signed S3 download redirects, and more. See
[all extensions](/docs/extensions).

## What stays the same

The admin dashboard, collections, rules, the REST and realtime APIs, JS hooks and migrations all work as in PocketBase.
For those, the [PocketBase docs](/docs/pocketbase) are the reference.
