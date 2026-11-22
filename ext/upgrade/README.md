---
owners: ["@benallfree"]
version: 0.0.4
title: "Upgrade"
group: binary
summary: "pocketone upgrade to the latest release, keeping the old executable."
---

# Upgrade

`pocketone upgrade` replaces the `pocketone` executable with the latest PocketOne release and keeps the old one beside
it as `<name>-<version>` (for example `pocketone-0.0.3`), so a project can keep running the engine it was built with.

## Install

Upgrade is built into `pocketone`: the `upgrade` command is always there. There's nothing to turn on.

In your own Go build, register the command before the app starts:

```go
import "pocketone.dev/ext/upgrade"

app := pocketbase.New()
upgrade.MustRegister(app, app.RootCmd, upgrade.Config{})
app.Start()
```

## Configure

Nothing to configure: no `pocketone.yaml` section, no environment variables, no settings screens. The command has one
flag:

| Flag | Default | What it does |
|---|---|---|
| `--backup` | `true` | Back up `pb_data` after upgrading, if the current folder has one. `--backup=false` skips it. |
| `--yes`, `-y` | `false` | Move from a stable release to a prerelease without asking. |

## Use

Run it from your project folder:

```bash
pocketone upgrade
```

It:

1. Picks the newest release on GitHub that fits what you're running, and stops if you already have it:
   - The newest release is offered, prereleases included: `0.1.0-alpha.2` goes to `0.1.0-alpha.3` or `0.1.0`,
     whichever is newest.
   - **From one prerelease to the next** (or to a stable release), it just upgrades.
   - **From a stable release to a prerelease** (`0.1.0` to `0.1.1-alpha.1`), it asks first. No means the newest stable
     release instead, if there's a newer one. `--yes` answers yes for you; with no answer (no terminal), it's no.
   - A local build upgrades to the newest release without asking.
2. Downloads the archive for your platform and checks it against the release's `checksums.txt` before touching
   anything.
3. Renames the current executable to `<name>-<version>` and puts the new one in its place. **Old copies are never
   deleted**, so `pocketone-0.0.3 serve` still runs a project on that version. A local build with no release version
   is kept as `<name>.old`, which the next such upgrade replaces.
4. Backs up `pb_data`, when the current folder has one, as `@upgrade_<tag>.zip` in PocketBase's backups.
5. Prints the new version, where the old one was kept, and the release notes.

Two cases where it does something else:

- **Installed with Homebrew:** it prints `brew update && brew upgrade pocketone` instead (`brew update` refreshes the tap, which Homebrew otherwise does only once a day). Homebrew keeps the previous version until
  `brew cleanup`.
- **In a Docker container:** it refuses; upgrade the image instead.

It needs write access to the folder the executable is in.

A kept executable doesn't undo migrations: once a newer version has served a `pb_data`, an older one may not open it.
The backup is the way back for that.

## Integrations

Nothing integrates with it yet. Other tools can reuse its release code:

- **Custom builds** that publish their own releases set `Owner`, `Repo`, `ArchivePrefix` and `ArchiveExecutable` in
  `upgrade.Config`. Release archives are named `<prefix>_<version>_<os>_<arch>.zip`, with a goreleaser-style
  `checksums.txt` in the same release.
- **Go API.** `Config.FetchReleases(ctx)` lists the published releases, prereleases included, and
  `upgrade.UpgradeFor(current, releases, stableOnly)` picks the one to upgrade to and `upgrade.NeedsConfirm(current, release)`
  says whether to ask first. `Config.FetchRelease(ctx, tag)`
  fetches GitHub's latest stable release (tag `""`) or a given one;
  `Config.DownloadArchive(ctx, release, goos, goarch, dir)` downloads and checks a platform's archive.
  `upgrade.KeptName(exe, version)` is the name an old executable is kept under.

## Example

[`example/main.go`](example/main.go) is a PocketOne app with just the `upgrade` command. Try it with `go run ./example upgrade`.

```go
// Command example is a PocketOne app with just the upgrade command. A local build has no release version, so it is
// kept as "example.old"; release builds are kept as "<name>-<version>".
//
//	go run ./example upgrade
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"pocketone.dev/ext/upgrade"
)

func main() {
	app := pocketbase.New()
	upgrade.MustRegister(app, app.RootCmd, upgrade.Config{})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## License

MIT
