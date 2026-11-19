---
owners: ["@benallfree"]
version: 0.0.2
title: "Desktop"
group: binary
summary: "Build your PocketOne app into a native desktop app for macOS, Windows and Linux, one executable each, from any machine."
---

# Desktop

The same app folder you run with `pocketone serve` becomes a desktop app: `pocketone desktop build` compiles your
`pb_hooks`, `pb_migrations` and `pb_public` into one executable that runs the app's server on a private `127.0.0.1`
port and shows it in a native window (the system webview, through [Wails](https://wails.io)). Your users install
nothing else, and each one's data stays on their machine.

All you need is `pocketone` and Docker: the build runs in a build image with Go and PocketOne's source, and every
platform builds from any machine, with no C compiler or Xcode. The desktop runtime uses Wails' cgo-free backends,
which Wails marks experimental on macOS and Linux.

## Install

Desktop is built into `pocketone`. Builds run in [Docker](https://docs.docker.com/get-docker/), in the build image
for your `pocketone`'s version (`ghcr.io/pocketonehq/pocketone-build:v<version>`), pulled on first use. It holds Go
and PocketOne's source with every dependency vendored, so once it's here builds work offline. A named volume
(`pocketone-build-cache`) keeps Go's build cache, so rebuilds take seconds.

A development `pocketone` (built from a checkout) builds its own image, `pocketone-build:dev`, from that checkout on
first use (`pocketone desktop image` rebuilds it). With Go 1.27 or newer and a checkout, `--local` skips Docker and
builds with your own Go (`--source` or `P1_SOURCE` names the checkout when you aren't in it).

## Configure

`pocketone.yaml`'s `desktop` section names and sizes the app:

```yaml
desktop:
  name: Notes              # window title and executable name; default: the folder's name
  id: dev.example.notes    # names each user's data folder and the single-instance lock; default: app.pocketone.<name>
  width: 900               # starting window size; default 1024 × 768
  height: 640
```

Pick the `id` once and keep it: it's where your users' data lives, so a new id is a new, empty app for them.

A `sign` section signs every build for the platforms it names (`--no-sign` skips it): macOS with your Developer ID,
notarized and packaged as a DMG; Windows with your signing command; Linux with a checksum and a GPG signature. See
[Signing and sharing](docs/signing.md).

```yaml
desktop:
  sign:
    macos:
      identity: "Developer ID Application: Your Name (ABCDE12345)"
      notary_profile: pocketone-notary
    windows:
      command: signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /a {file}
    linux:
      gpg_key: you@example.com
```

## Use

In your app's folder:

```sh
pocketone serve            # the app as a normal server, as always
pocketone desktop build    # a desktop app for this machine, in dist/
pocketone desktop build --all                                  # macOS, Windows and Linux, arm64 and amd64
pocketone desktop build --target windows/amd64,linux/amd64     # some of them
```

`dist/<os>-<arch>/` gets `<name>.app` on macOS (signed ad hoc when you build on a Mac, so it runs there), `<name>.exe` on
Windows (no console window), and `<name>` on Linux.

What the desktop app does when it starts:

- **Its data** goes in the user's own app-data folder, by `id`: `~/Library/Application Support/<id>/pb_data` on
  macOS, `%AppData%\<id>\pb_data` on Windows, `~/.config/<id>/pb_data` on Linux.
- **Your migrations** run on every start, so a new version of your app upgrades each user's data.
- **Your hooks and public files** are read from the executable, never from disk.
- **It serves** on a free `127.0.0.1` port and opens the window on it. Only one copy runs at a time (Wails' single-instance lock, by `id`).
- **There is no superuser** and the installer page never opens; the app's users aren't its admins.

What each platform needs:

- **macOS:** 11 or newer. Apps you send to others need signing and notarization with your own Apple Developer ID;
  unsigned, macOS asks them to approve the app in System Settings first.
- **Windows:** the WebView2 runtime, which Windows 10 and 11 already have.
- **Linux:** the GTK 4 and WebKitGTK 6.0 libraries; without them the app says which package to install.

## Integrations

| Extension | How they fit |
|---|---|
| [Bundle](../bundle/README.md) | The desktop app serves your hooks, migrations and public files from the executable through Bundle. |
| [Standard](../standard/README.md) | The desktop app runs the standard set, as `pocketone serve` does. |
| [JSVM](../jsvm/README.md) | Your hooks run in PocketOne's QuickJS engine. |

### Building on Desktop

The runtime is its own module, `ext/desktop/runtime`, so the `pocketone` binary never links Wails. Your own Go
program can use it directly:

```go
desktop.Run(desktop.App{Name: "Notes", ID: "dev.example.notes", Hooks: hooks, Migrations: migrations, Public: public})
```

Build it with `CGO_ENABLED=0 go build -tags purego` for macOS and Linux (Wails' cgo-free backends); Windows needs no
tag. The build image is [image/Dockerfile](image/Dockerfile): a `pocketone` and the vendored module `pocketone desktop
prepare` writes.

## Example

[example/hello](example/hello) is a hello-world app: a hook (`GET /api/hello`), a migration (a `greetings`
collection with one row) and a page that lists and adds greetings.

```sh
cd ext/desktop/example/hello
pocketone superuser upsert admin@example.com password
pocketone serve                 # http://127.0.0.1:8090
pocketone desktop build         # dist/darwin-arm64/Hello.app on an Apple Silicon Mac
```

[example/main.go](example/main.go) is a PocketBase app with only this extension, to show the command.

```go
// Command example is a PocketBase app with only the desktop extension: `go run . desktop build` in an app folder
// (one with pb_hooks, pb_migrations or pb_public, like ./hello) builds it as a desktop app. The pocketone binary
// has the same command.
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"

	clicmdv1 "pocketone.dev/ext/contracts/cli/commands/v1"
	"pocketone.dev/ext/desktop"
	"pocketone.dev/ext/extkit"
)

func main() {
	app := pocketbase.New()
	if _, err := extkit.Setup(app, []extkit.Extension{desktop.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	clicmdv1.Mount(app, app.RootCmd) // the commands extensions provide; the standard set does this for pocketone
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## License

MIT
