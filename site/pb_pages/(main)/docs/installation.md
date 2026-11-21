---
title: Installation
description: Install PocketOne with Homebrew, Scoop, a release download or go build, and keep it up to date.
---

# Installation

PocketOne is one executable, `pocketone`. It is built on PocketBase v0.42 and takes the same commands and flags, plus its own.

## Homebrew (macOS and Linux)

```bash
brew install pockethost/tap/pocketone
```

The formula lives in [pockethost/homebrew-tap](https://github.com/pockethost/homebrew-tap).

## Scoop (Windows)

```powershell
scoop bucket add pockethost https://github.com/pockethost/scoop-bucket
scoop install pocketone
```

That's [Scoop](https://scoop.sh); the manifest lives in [pockethost/scoop-bucket](https://github.com/pockethost/scoop-bucket).
To install without Scoop, see [Install on Windows](/docs/windows).

## Download a release

Download the zip for your platform from
[Releases](https://github.com/pocketonehq/pocketone/releases), unzip it, and put `pocketone` somewhere on your `PATH`.

## Build from source

With [Go](https://go.dev) installed:

```bash
git clone https://github.com/pocketonehq/pocketone.git
cd pocketone/examples/pocketone
go build -o pocketone .
```

## Run it

Create your first superuser before the first serve:

```bash
pocketone superuser upsert <email> <password>
pocketone serve
```

Opening `http://127.0.0.1:8090/_/` to create it also works, as in PocketBase.

To start a site with server-rendered pages:

```bash
pocketone pages create myapp
cd myapp
pocketone serve --dev --dir ./pb_data
```

In dev mode (`serve --dev`), an app with no SMTP settings doesn't send mail through the system's `sendmail`. PocketOne
catches the mail and shows it at `http://127.0.0.1:8025`.

## Upgrade

```bash
pocketone upgrade
```

Installed with Homebrew or Scoop? Upgrade with `brew upgrade pocketone` or `scoop update pocketone` instead.

This replaces the executable with the latest release. The old one is kept beside it as `pocketone-<version>`, never
deleted, so you can go back.

## Moving from PocketBase

PocketOne reads the same `pb_data`, `pb_hooks`, `pb_migrations` and `pb_public` folders. Point it at your existing data
directory with `--dir`, as you would with `pocketbase`. Back up `pb_data` first.
