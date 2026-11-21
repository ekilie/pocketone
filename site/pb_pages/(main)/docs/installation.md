---
title: Installation
description: Install PocketOne on macOS, Windows or Linux with Homebrew, Scoop or the install script, run it, and keep it up to date.
---

# Installation

PocketOne is one executable, `pocketone`. It is built on PocketBase and takes the same commands and flags, plus its own.

- **Install:** [macOS](#macos) · [Windows](#windows) · [Linux](#linux)
- **Use:** [Run it](#run-it) · [Upgrade](#upgrade) · [Moving from PocketBase](#moving-from-pocketbase)

Another way to install (a release zip, Windows without Scoop, building from source, the install script's settings)?
See [Advanced installation](/docs/guides/advanced-installation).

## macOS

With [Homebrew](https://brew.sh):

```bash
brew install pocketonehq/tap/pocketone
```

The formula lives in [pocketonehq/homebrew-tap](https://github.com/pocketonehq/homebrew-tap). Without Homebrew, use
the install script:

```bash
curl -fsSL https://pocketone.dev/install.sh | sh
```

It picks the newest release for your Mac, checks its checksum and puts `pocketone` in `/usr/local/bin` (asking for your
password through `sudo` when that folder isn't yours). Read it first if you like: [pocketone.dev/install.sh](/install.sh).

## Windows

With [Scoop](https://scoop.sh), in PowerShell:

```powershell
scoop bucket add pocketonehq https://github.com/pocketonehq/scoop-bucket
scoop install pocketone
```

The manifest lives in [pocketonehq/scoop-bucket](https://github.com/pocketonehq/scoop-bucket). Without Scoop, see
[Windows without Scoop](/docs/guides/advanced-installation#windows-without-scoop).

## Linux

With [Homebrew](https://brew.sh), as on macOS:

```bash
brew install pocketonehq/tap/pocketone
```

Or with the install script, which suits a server:

```bash
curl -fsSL https://pocketone.dev/install.sh | sh
```

It finds your package manager (apt, dnf, yum, zypper or apk), downloads the newest release's `.deb`, `.rpm` or `.apk`
package for your system, checks its checksum and installs it with `sudo`. The package also sets PocketOne up as a
systemd service, stopped until you start it. With none of those package managers it puts the plain binary in
`/usr/local/bin` instead. To set the service up, see [Run PocketOne on a Linux server](/docs/guides/linux-server).

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

Upgrade the way you installed:

| Installed with | Upgrade with |
|---|---|
| Homebrew | `brew upgrade pocketone` |
| Scoop | `scoop update pocketone` |
| The install script | Run it again. |
| Anything else | `pocketone upgrade` |

`pocketone upgrade` replaces the executable with the latest release. The old one is kept beside it as
`pocketone-<version>` (`pocketone-<version>.exe` on Windows), never deleted, so you can go back. For a Homebrew, Scoop
or Linux package install it tells you the command above instead.

## Moving from PocketBase

PocketOne reads the same `pb_data`, `pb_hooks`, `pb_migrations` and `pb_public` folders. Point it at your existing data
directory with `--dir`, as you would with `pocketbase`. Back up `pb_data` first.
