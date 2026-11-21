---
title: Installation
description: Install PocketOne on macOS, Linux or Windows with the install script, Homebrew, a Linux package, a release download or go build, and keep it up to date.
---

# Installation

PocketOne is one executable, `pocketone`. It is built on PocketBase v0.42 and takes the same commands and flags, plus its own.

## Install script (macOS and Linux)

```bash
curl -fsSL https://pocketone.dev/install.sh | sh
```

It picks the newest release for your system and checks its checksum. On macOS it puts `pocketone` in
`/usr/local/bin`. On Linux it installs the release's `.deb`, `.rpm` or `.apk` package, which also sets PocketOne up
as a systemd service ([Linux](/docs/linux)); on a system with none of those package managers it installs the binary
like on macOS. It asks for your password through `sudo` when it needs to write there. Run it again to upgrade.

Settings go before `sh`:

```bash
curl -fsSL https://pocketone.dev/install.sh | POCKETONE_VERSION=v0.0.1-alpha.10 sh
```

| Variable | What it does |
|---|---|
| `POCKETONE_VERSION` | Install this release tag instead of the newest one. |
| `POCKETONE_INSTALL_DIR` | Where the binary goes when no package is used. Default `/usr/local/bin`; a folder you own, such as `$HOME/.local/bin`, needs no `sudo`. |
| `POCKETONE_NO_PACKAGE=1` | On Linux, install the plain binary, never a package. |

Read the script first if you like: [pocketone.dev/install.sh](/install.sh).

## Homebrew (macOS and Linux)

```bash
brew install pockethost/tap/pocketone
```

The formula lives in [pockethost/homebrew-tap](https://github.com/pockethost/homebrew-tap).

## Linux packages

Every release has `.deb` (Debian, Ubuntu), `.rpm` (Fedora, RHEL, Rocky, Alma, openSUSE) and `.apk` (Alpine)
packages for `amd64`, `arm64`, `armv7`, `ppc64le` and `s390x`. The `.deb` and `.rpm` add a systemd service that runs
as its own `pocketone` user. [Linux](/docs/linux) covers installing one by hand, setting up the service, HTTPS and
upgrades.

## Download a release

Download the zip for your platform from
[Releases](https://github.com/pocketonehq/pocketone/releases), unzip it, and put `pocketone` somewhere on your `PATH`.
This is the way on Windows. Each release's `checksums.txt` lists the SHA-256 of every file.

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

To run it on a Linux server as a service, see [Linux](/docs/linux).

## Upgrade

Upgrade the way you installed:

| Installed with | Upgrade with |
|---|---|
| The install script | Run it again. |
| Homebrew | `brew upgrade pocketone` |
| A Linux package | The install script, or install the newer package ([Linux](/docs/linux#upgrade)). |
| A release zip or `go build` | `pocketone upgrade` |

`pocketone upgrade` replaces the executable with the latest release. The old one is kept beside it as
`pocketone-<version>`, never deleted, so you can go back. Use it for the binary the install script put in
`/usr/local/bin` too (with `sudo` if that folder is root's), but not for a Homebrew or package install: those belong
to their package manager.

## Moving from PocketBase

PocketOne reads the same `pb_data`, `pb_hooks`, `pb_migrations` and `pb_public` folders. Point it at your existing data
directory with `--dir`, as you would with `pocketbase`. Back up `pb_data` first.
