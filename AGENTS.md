# PocketOne

PocketOne is PocketBase plus plugins. **PocketBase's own code ("core") changes only for hooks that let `ext/`
extend or override behavior; the features themselves live in `ext/`.** Core is every path outside what PocketOne owns:
`ext/`, `site/`, `examples/pocketone/`, `docs/`, `README.md`, `AGENTS.md`, `CLAUDE.md`, `.github/`, `.claude/`. Each core change is marked
`// PocketOne:` so upstream merges stay easy.

## Every core change gets an independent audit before it reaches main

Any commit to `main` that changes core must pass an audit by a reviewer who didn't write it (a fresh agent with no
context from the change's session, or a human), and must record the result in a `Core-Audit:` commit trailer. CI
(`.github/workflows/core-audit.yml`) fails commits that change core without one.

1. Get the core diff: `.github/core-audit/core-diff.sh origin/main HEAD` (or `<sha>^ <sha>` for one commit).
2. Give the reviewer only that diff, this section, and the commit message. For each changed hunk it answers:
   - **Necessary:** could this live in `ext/` instead, using hooks core already has (upstream's own, or PocketOne's existing
     seams such as `OnFilesystemCreate`, `jsvm.Config.Extensions`, `BaseAppConfig.AppMigrations`/`OnLog`)?
   - **Minimal:** is it the smallest seam that works, with no feature logic, policy or plugin-specific names in core?
   - **Sensible:** marked `// PocketOne:`, documented on the hook, upstream behavior unchanged when nothing uses it, tested?
   - **Listed:** a new or changed hook has its row in the README's "Changes we made to PocketBase" Hooks table (what it
     lets extensions do, which extensions use it); a bug fix has its row in the Bugfixes table.
3. Pass: add the trailer and push. Fail: move the logic to `ext/` or shrink the seam, then audit again.

Trailer forms (the last line of the commit message, after a blank line):

```
Core-Audit: pass - <one line: what the core change is and why it can't live in ext/>
Core-Audit: upstream-merge v0.x.y
```

`upstream-merge` is only for merging a PocketBase release; resolve conflicts in the `// PocketOne:` spots and audit any
PocketOne change the merge forced.

## The `core-changes` branch

`core-changes` is upstream `develop` plus one commit holding every core change on `main` (no `ext/` or other
PocketOne-owned paths), so anyone can audit our core changes against current upstream in one diff. It is rebuilt, never
merged: after every upstream pull run `git fetch upstream origin && .github/core-audit/core-changes-branch.sh`, then
`git push --force origin core-changes`, and commit `README.md` if the script repinned its audit link. A conflict stops the script with the worktree left to resolve
(`--continue` finishes); resolutions are remembered (rerere) for later runs.

## Core changes that aren't hooks

Bug fixes to upstream behavior, each tested and meant for upstream; drop each one once upstream has it:

- `apis/middlewares.go` `logRequest`: logs the status the client actually got for a plain error, not 0.
- `apis/realtime.go` `realtimeCanAccessRecord`: a superuser's subscription filter may use hidden fields, as in the
  list API (`TestRealtimeSuperuserFilterOnHiddenField`).
- `core/notify_watcher.go`: keeps the notify file until the next write, so macOS (kqueue) sees the change.
- `core/notify_watcher.go`: stops the watcher goroutine (and waits for it) before `ClearBootstrap`, which it raced
  (`TestNotifyWatcher_StoppedBeforeClearBootstrap`, `TestNotifyWatcher_QueuedReloadAfterClearBootstrap`).

## Other rules

- Plugins are their own Go modules under `ext/<name>/` with a README and a minimal `example/main.go`; a `multisite-`
  name means it only works under `ext/multisite`
  (`docs/plugins.md`). Modules that reach another `ext/` module add a `replace` for it.
- Plugin CI (`.github/workflows/plugins.yml`, tool in `.github/plugins/`): every `pluginkit.Plugin` module has a
  conformance test, and each module a change affects builds, vets and passes `go test -race`. Check locally with
  `go run ./.github/plugins check` and `go run ./.github/plugins affected origin/main HEAD`.
- pocketone.dev (`site/`) copies plugin READMEs and `docs/plugins.md`, and indexes every page for search. A new plugin
  gets an entry in `site/plugins.json`; after changing a README, `docs/plugins.md` or any site page, run
  `cd site/sync && go run .` (CI checks it).
- Build checks: `go build -o /dev/null ./...` (a plain `go build` in a single-main module writes a binary into the
  tree). Stage files by name; never commit built binaries.
- Plans and internal notes go in a private maintainer repo, never here.
