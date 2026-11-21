---
title: Installation
description: Install PocketOne on macOS, Windows or Linux (Homebrew, Scoop, the install script, a Linux package, a release download or go build), run it, set it up as a Linux service, and keep it up to date.
---

# Installation

PocketOne is one executable, `pocketone`. It is built on PocketBase v0.42 and takes the same commands and flags, plus its own.

- **Install:** [macOS](#macos) · [Windows](#windows) · [Linux](#linux) · [Other ways](#other-ways-to-install) · [Install script settings](#install-script-settings)
- **Use:** [Run it](#run-it) · [Linux server](#linux-server) · [Upgrade](#upgrade) · [Moving from PocketBase](#moving-from-pocketbase)

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

The manifest lives in [pocketonehq/scoop-bucket](https://github.com/pocketonehq/scoop-bucket).

### Windows without Scoop

Releases include 64-bit Windows builds: the zip ending in `_windows_amd64.zip` for most PCs, `_windows_arm64.zip` for
ARM ones. The executable inside is `pocketone.exe`. Download one from
[Releases](https://github.com/pocketonehq/pocketone/releases), then in PowerShell, from the folder you downloaded it to:

```powershell
$dir = "$env:LOCALAPPDATA\Programs\pocketone"
Expand-Archive pocketone_*_windows_amd64.zip -DestinationPath $dir -Force
Unblock-File "$dir\pocketone.exe"
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$dir", "User")
```

This puts PocketOne in a folder of your own, so it needs no administrator rights, and adds that folder to your user
`PATH`. Open a new terminal and check it with `pocketone --version`.

`Unblock-File` clears the "downloaded from the internet" mark. The executable isn't code-signed, so if you start it
from File Explorer instead, SmartScreen may stop it: choose **More info**, then **Run anyway**. The commands on this
page are the same in PowerShell and in Command Prompt.

## Linux

The install script:

```bash
curl -fsSL https://pocketone.dev/install.sh | sh
```

It finds your package manager (apt, dnf, yum, zypper or apk), downloads the newest release's `.deb`, `.rpm` or `.apk`
package for your system, checks its checksum and installs it with `sudo`. The package also sets PocketOne up as a
systemd service, stopped until you start it ([Linux server](#linux-server)). With none of those package managers it
puts the plain binary in `/usr/local/bin` instead.

[Homebrew](https://brew.sh) works on Linux too, for the binary alone: `brew install pocketonehq/tap/pocketone`.

PocketOne's Linux builds are static binaries for `amd64`, `arm64`, `armv7` (32-bit ARM, such as older Raspberry Pis),
`ppc64le` and `s390x`, so it runs on any distribution.

## Other ways to install

### Download a release

Download the zip for your platform from [Releases](https://github.com/pocketonehq/pocketone/releases), unzip it, and
put `pocketone` somewhere on your `PATH`. Each release's `checksums.txt` lists the SHA-256 of every file.

### Build from source

With [Go](https://go.dev) installed:

```bash
git clone https://github.com/pocketonehq/pocketone.git
cd pocketone/examples/pocketone
go build -o pocketone .
```

## Install script settings

The script reads these environment variables. With `curl | sh`, put them before `sh`:

```bash
curl -fsSL https://pocketone.dev/install.sh | POCKETONE_VERSION=v0.0.1-alpha.10 sh
```

| Variable | What it does |
|---|---|
| `POCKETONE_VERSION` | Install this release tag instead of the newest one. |
| `POCKETONE_INSTALL_DIR` | Where the binary goes when no package is used. Default `/usr/local/bin`; a folder you own, such as `$HOME/.local/bin`, needs no `sudo`. |
| `POCKETONE_NO_PACKAGE=1` | On Linux, install the plain binary, never a package. |

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

## Linux server

On a server, install PocketOne as a package ([Linux](#linux)): you get a systemd service running as its own
`pocketone` user, ready to start once you've set it up.

### Install a package by hand

Every [release](https://github.com/pocketonehq/pocketone/releases) has `.deb` (Debian, Ubuntu), `.rpm` (Fedora, RHEL,
Rocky, Alma, openSUSE) and `.apk` (Alpine) packages, named `pocketone_<version>_linux_<arch>.<deb|rpm|apk>`, and a
`checksums.txt` with the SHA-256 of each. To install one:

```bash
# Debian, Ubuntu
sudo apt install ./pocketone_<version>_linux_amd64.deb

# Fedora, RHEL, Rocky, Alma
sudo dnf install ./pocketone_<version>_linux_amd64.rpm

# openSUSE
sudo zypper install --allow-unsigned-rpm ./pocketone_<version>_linux_amd64.rpm

# Alpine
sudo apk add --allow-untrusted ./pocketone_<version>_linux_amd64.apk
```

The packages aren't signed; check them against `checksums.txt` (`sha256sum -c checksums.txt --ignore-missing`).

### What a package installs

| Path | What it is |
|---|---|
| `/usr/bin/pocketone` | The binary. |
| `/usr/lib/systemd/system/pocketone.service` | The service (`.deb` and `.rpm`; Alpine uses OpenRC, so its package has the binary and config only). |
| `/etc/pocketone/pocketone.yaml` | The service's [pocketone.yaml](/docs/pocketone-yaml): data in `/var/lib/pocketone`. Kept when you upgrade. |
| `/etc/pocketone/service.env` | What the service passes to `pocketone serve`: your domain. Kept when you upgrade. |
| `/var/lib/pocketone` | The `pocketone` user's home, holding `pb_data` and your app's `pb_hooks`, `pb_migrations`, `pb_public` and `pb_pages`. |

Installing makes the `pocketone` system user and leaves the service stopped.

### Run it as a service

Create your first superuser, as the `pocketone` user so the files it makes are that user's:

```bash
sudo -u pocketone pocketone superuser upsert <email> <password> --config /etc/pocketone/pocketone.yaml
```

Run any other `pocketone` command for the service's app the same way, with `sudo -u pocketone` and
`--config /etc/pocketone/pocketone.yaml`.

Then choose how it's reached, in `/etc/pocketone/service.env`:

- **Its own HTTPS.** Put your domain in `SERVE_ARGS`, after pointing the domain's DNS at the server:

  ```bash
  SERVE_ARGS=example.com
  ```

  `serve` then listens on ports 80 and 443 and gets a Let's Encrypt certificate for the domain by itself, as in
  PocketBase. List more domains with spaces between them.

- **Behind a reverse proxy** (Caddy, nginx) that handles HTTPS: leave `SERVE_ARGS` empty. PocketOne listens on
  `127.0.0.1:8090`; point the proxy there.

Start it now and at every boot, and follow its log:

```bash
sudo systemctl enable --now pocketone
journalctl -u pocketone -f
```

After changing `service.env` or `pocketone.yaml`, restart it: `sudo systemctl restart pocketone`. Secrets for
`pocketone.yaml` go in `/etc/pocketone/.env`, written in the yaml as `$NAME`; make it readable by the `pocketone`
user only (`sudo chown root:pocketone /etc/pocketone/.env && sudo chmod 640 /etc/pocketone/.env`).

To deploy your app, put its folders in `/var/lib/pocketone`, owned by the `pocketone` user, and restart the service:

```bash
sudo cp -r pb_hooks pb_migrations pb_public /var/lib/pocketone/
sudo chown -R pocketone:pocketone /var/lib/pocketone
sudo systemctl restart pocketone
```

Or push it from your machine with `pocketone deploy` ([Deploy and live sync](/docs/features/deploy)).

### A service without a package

With the plain binary in `/usr/local/bin` (the install script with `POCKETONE_NO_PACKAGE=1`, or a release zip), set
the service up yourself. Make the user and data folder, and the first superuser:

```bash
sudo useradd --system --user-group --home-dir /var/lib/pocketone --create-home --shell /usr/sbin/nologin pocketone
sudo -u pocketone pocketone superuser upsert <email> <password> --dir /var/lib/pocketone/pb_data
```

Save this as `/etc/systemd/system/pocketone.service`, with your domain in place of `example.com` (or, behind a
reverse proxy, no domain and no `AmbientCapabilities` line):

```ini
[Unit]
Description=PocketOne
After=network-online.target
Wants=network-online.target

[Service]
User=pocketone
Group=pocketone
WorkingDirectory=/var/lib/pocketone
ExecStart=/usr/local/bin/pocketone serve example.com --dir /var/lib/pocketone/pb_data --publicDir /var/lib/pocketone/pb_public
Restart=on-failure
RestartSec=5s
LimitNOFILE=4096
# lets the pocketone user listen on ports 80 and 443
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
```

Then:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pocketone
```

### Uninstall

```bash
sudo apt remove pocketone     # or: sudo dnf remove pocketone, sudo zypper remove pocketone, sudo apk del pocketone
```

This stops the service and removes the binary and the service. Your data in `/var/lib/pocketone` and the `pocketone`
user stay; delete them yourself if you're sure (`sudo rm -rf /var/lib/pocketone && sudo userdel pocketone`).

## Upgrade

Upgrade the way you installed:

| Installed with | Upgrade with |
|---|---|
| Homebrew | `brew upgrade pocketone` |
| Scoop | `scoop update pocketone` |
| The install script | Run it again. |
| A Linux package | The install script, or install the newer package by hand. The package manager restarts the service if it's running and keeps your `/etc/pocketone` files and your data. |
| A release zip or `go build` | `pocketone upgrade` |

`pocketone upgrade` replaces the executable with the latest release. The old one is kept beside it as
`pocketone-<version>` (`pocketone-<version>.exe` on Windows), never deleted, so you can go back. Use it for the binary
the install script put in `/usr/local/bin` too (with `sudo` if that folder is root's, then
`sudo systemctl restart pocketone` if it runs as a service). For a Homebrew, Scoop or Linux package install it tells
you the command above instead.

## Moving from PocketBase

PocketOne reads the same `pb_data`, `pb_hooks`, `pb_migrations` and `pb_public` folders. Point it at your existing data
directory with `--dir`, as you would with `pocketbase`. Back up `pb_data` first.
