# PocketOne

PocketOne is PocketBase plus extensions. **PocketBase's own code ("core") changes only for hooks that let `ext/`
extend or override behavior; the features themselves live in `ext/`.** Core is every path outside what PocketOne owns:
`ext/`, `site/`, `examples/pocketone/`, `docs/`, `README.md`, `CONTRIBUTING.md`, `AGENTS.md`, `CLAUDE.md`, `.github/`, `.claude/`, `.claude-plugin/`, `agent/`. Each core change is marked
`// PocketOne:` so upstream merges stay easy.

Words: everything in `ext/` is an **extension**, never a "plugin". **Standard extensions** ship in every `pocketone`
binary (`examples/pocketone`); the rest of `ext/` are **community extensions**. "Plugin" means only upstream's
`plugins/` (jsvm, migratecmd, ghupdate) and PocketPages' own plugin system in `ext/pages` (`+config.js` `plugins`).

## Every core change gets an independent audit before it reaches main

Any commit to `main` that changes core must pass an audit by a reviewer who didn't write it (a fresh agent with no
context from the change's session, or a human), and must record the result in a `Core-Audit:` commit trailer. CI
(`.github/workflows/core-audit.yml`) fails commits that change core without one.

1. Get the core diff: `.github/core-audit/core-diff.sh origin/main HEAD` (or `<sha>^ <sha>` for one commit).
2. Give the reviewer only that diff, this section, and the commit message. For each changed hunk it answers:
   - **Necessary:** could this live in `ext/` instead, using hooks core already has (upstream's own, or PocketOne's existing
     seams such as `OnFilesystemCreate`, `BaseAppConfig.AppMigrations`/`OnLog`, or the `ext/jsvm` engine's extension methods)?
   - **Minimal:** is it the smallest seam that works, with no feature logic, policy or extension-specific names in core?
   - **Sensible:** marked `// PocketOne:`, documented on the hook, upstream behavior unchanged when nothing uses it, tested?
   - **Listed:** a new or changed hook is on its `core-hook/<name>` branch (a new one is added to `HOOK_STACK`) and has
     its row in the README's "Changes we made to PocketBase" Hooks table (what it lets extensions do, which extensions
     use it, its diff); a bug fix has its `core-bugfix/<name>` branch and its row in the Bugfixes table.
3. Pass: add the trailer and push. Fail: move the logic to `ext/` or shrink the seam, then audit again.

Trailer forms (the last line of the commit message, after a blank line):

```
Core-Audit: pass - <one line: what the core change is and why it can't live in ext/>
Core-Audit: upstream-merge v0.x.y
```

`upstream-merge` is only for merging a PocketBase release; resolve conflicts in the `// PocketOne:` spots and audit any
PocketOne change the merge forced.

## Branches that show our core changes

Three kinds, all on upstream `master` (PocketBase's release line; `develop` is unstable and never a base) and code only
(no `_test.go` files), rebuilt and force-pushed (never merged) by `.github/core-audit/core-changes-branch.sh` after every
upstream pull and after a core change lands on main:

- `core-changes`: upstream `master` plus one commit holding every core change on `main` (no `ext/` or other
  PocketOne-owned paths), to audit all of it in one diff.
- `core-hook/<name>`: one hook each, stacked in the script's `HOOK_STACK` order (each on the one before), so its diff
  against the previous branch shows just that hook.
- `core-bugfix/<name>`: one upstream bug fix each, written for upstream (no `// PocketOne:` markers).

The script ends with a sum check: the hook stack plus every bugfix branch must match `core-changes` in code (comments
may differ), so a core change on `main` without its branch fails it. Run
`git fetch upstream origin && .github/core-audit/core-changes-branch.sh`, then
`git push --force origin core-changes 'refs/heads/core-hook/*' 'refs/heads/core-bugfix/*'`, and commit `README.md` if
the script repinned its compare links. Conflicts stop the script with the worktree left to resolve; resolutions are
remembered (rerere) for later runs.

The `upstream` remote is this repo's own `upstream/develop` and `upstream/master` branches, a copy of PocketBase's
branches, so day-to-day fetches never reach PocketBase's server. Set it up once per clone:

```
git remote add upstream git@github.com:pockethost/pocketone.git
git config remote.upstream.fetch '+refs/heads/upstream/*:refs/remotes/upstream/*'
git config remote.upstream.tagopt --no-tags
```

`.github/workflows/upstream-mirror.yml` refreshes those branches from
`https://ganigeorgiev.com/git/pocketbase/pocketbase` once a day (fast-forward only; run it now with
`gh workflow run upstream-mirror`). Never fetch PocketBase's server from your own machine. If the workflow fails
because upstream rewrote history, mirror by hand from a machine that can reach it: fetch into
`refs/remotes/upstream/*`, then
`git push --force origin refs/remotes/upstream/develop:refs/heads/upstream/develop refs/remotes/upstream/master:refs/heads/upstream/master`.

## Syncing with upstream

Merge `upstream/master` (a PocketBase release) into `main` on a side branch in its own worktree, never on `main`
directly, and review it before it lands:

1. `git worktree add -b upstream-sync/vX.Y.Z <dir> origin/main`, then `git merge --no-ff --no-commit upstream/master`.
   Resolve conflicts at the `// PocketOne:` spots.
2. Review what changed (`git log --oneline <old base>..upstream/master` and upstream's `CHANGELOG.md`): for each change,
   what it means for our hooks, extensions and contracts, and fix in `ext/` what it breaks.
3. Check each bug fix in "Core changes that aren't hooks": if upstream now fixes it, drop ours (and its
   `core-bugfix/<name>` branch and README row); if upstream changed the code around it, keep ours only for what is still
   broken.
4. `go mod tidy` in every module (upstream's dependency bumps reach them), build, vet, and run the full test suite.
5. Commit with `Core-Audit: upstream-merge vX.Y.Z`, add the **PocketBase** line to `docs/CHANGELOG.md`, rebuild the
   core branches with `core-changes-branch.sh`, then merge the side branch into `main`.

## Core changes that aren't hooks

Bug fixes to upstream behavior, each tested and meant for upstream; drop each one once upstream has it:

- `apis/middlewares.go` `logRequest`: logs the status the client actually got for a plain error, not 0.
- `apis/realtime.go` `realtimeCanAccessRecord`: a superuser's subscription filter may use hidden fields, as in the
  list API (`TestRealtimeSuperuserFilterOnHiddenField`).
- `core/notify_watcher.go`: keeps the notify file until the next write, so macOS (kqueue) sees the change.
- `core/notify_watcher.go`: stops the watcher goroutine (and waits for it) before `ClearBootstrap`, which it raced
  (`TestNotifyWatcher_StoppedBeforeClearBootstrap`, `TestNotifyWatcher_QueuedReloadAfterClearBootstrap`).
- `tools/cron/cron.go` `Start`: the ticker goroutine keeps its own ticker, since `Stop` (buffered `tickerDone`
  since v0.42) no longer waits for it before setting `c.ticker` to nil (`TestCronStopDoesNotRaceTicker`).

## Writing an extension

Read [docs/contracts.md](docs/contracts.md) (the idea) and [docs/extensions.md](docs/extensions.md) (the code) first.
Extensions meet only through **contracts**: named, versioned Go interfaces in `ext/contracts/<id path>/vN/`, listed in
the catalogue [ext/contracts/README.md](ext/contracts/README.md). Design the contracts before the extension, in this
order of preference:

1. **Use an existing contract.** Search the catalogue first. If a contract already promises what your extension
   needs, put it in `Requires` or `WorksWith`. If one already promises what your extension offers, provide that one:
   your extension then stands in for the current provider (a default steps aside; otherwise `providers:` in
   `pocketone.yaml` picks).
2. **Extend an existing contract.** These leave its promise unchanged:
   - Provide it alongside others, for contracts that take a chain (`mail.fallback/v1`).
   - Decorate whatever provides it (`Contract.Wrap`, declared in `Wraps`).
   - Add or replace `pocketone` commands through `cli.commands/v1`.
   - Give it a dashboard screen (`Contract.UI`).

   If the promise itself has to change, add `vN+1` beside `vN`, reviewed with the contract's owners. Never change what
   a published `vN` promises.
3. **Invent a new contract only when none fits.** In the commit message, name the contracts you checked and say why
   none fits. Keep it small and name it for what it does (`restore/v1`), never for your extension. A new optional
   ability next to an existing contract is a small companion contract (`mail.suppression/v1` beside `bulkmail/v1`),
   not a second copy of the contract with changes. Add it with its README (front matter `owners`, `summary`; sections
   `Use`, `Provide`, `Extensions`, `License`), per
   [Add a contract](ext/contracts/README.md#add-a-contract). Contracts are reviewed like core: every provider and
   caller relies on the promise.

Rules every extension follows:

- **Every API a client can call is a contract operation** under `/api/_c/<id>/`. Extensions register no routes of their
  own; only paths a standard fixes stay at the root (`/.well-known/*`, `/mcp`).
- **Never import another extension's module** (tests and examples may). Anything shared, such as helpers, scopes or a
  client, belongs in the contract package.
- **Declare by contract id, never by extension name**, in `Contracts()` (`Implements`, `Defaults`, `Wraps`, `Requires`,
  `WorksWith`). Provide in `Register` (`Provide` / `ProvideDefault`); look contracts up only after registration
  (`Of`, `Must` in hooks or at serve time). A lookup inside `Register` panics.
- **After changing a contract interface**, run `go generate ./...` in `ext/contracts`. Then run
  `go run ./.github/extensions contracts` to regenerate the catalogue and each contract's Extensions section, and
  `go run ./.github/extensions check`. CI fails if either is stale.

## Other rules

- Extensions are their own Go modules under `ext/<name>/` with a README and a minimal `example/main.go`
  (`docs/extensions.md`). Modules that reach another `ext/` module add a `replace` for it.
- Extension CI (`.github/workflows/extensions.yml`, tool in `.github/extensions/`): every `extkit.Extension` module has a
  conformance test, and each module a change affects builds, vets and passes `go test -race`. Check locally with
  `go run ./.github/extensions check` and `go run ./.github/extensions affected origin/main HEAD`.
- pocketone.dev (`site/`) copies extension READMEs and `docs/extensions.md`, and indexes every page for search. A new extension
  gets an entry in `site/extensions.json`; after changing a README, `docs/extensions.md` or any site page, run
  `cd site/sync && go run .` (CI checks it).
- Agent Skills (`agent/`, listed in `.claude-plugin/marketplace.json`): `site/sync` also writes each skill's `reference/`
  folder and both sites' `llms.txt` / `llms-full.txt` from the docs pages. The `SKILL.md` files are hand-written: a change
  that makes one wrong (a command, a `pocketone.yaml` key, an extension, a PocketPages API) fixes it in the same commit.
- JS types: after a core change that touches the JS API (a hook, a type JS sees) or an edit to `ext/jsvm/pocketone.d.ts`,
  run `go generate` in `ext/jsvm` to rebuild `ext/jsvm/types.d.ts` (its `TestTypes` tests fail until you do). An
  extension with a JS API declares it in a `TypeScript()` method.
- Environment variables are named for the layer that reads them: `PB_` is PocketBase core's (leave those alone),
  `P1_` is PocketOne's in every build (`extkit.EnvVar("TOKEN")` is `P1_TOKEN`), and a build on top uses its own
  prefix for its own variables. Never name a variable after the binary.
- Build checks: `go build -o /dev/null ./...` (a plain `go build` in a single-main module writes a binary into the
  tree). Stage files by name; never commit built binaries.
- **Changelog:** every commit to `main` with a user-visible change adds its line under `## Unreleased` in
  [docs/CHANGELOG.md](docs/CHANGELOG.md), in the same commit (policy at its top; breaking changes need an
  **Upgrading** step). Release notes are that file's section for the version, printed by
  `examples/pocketone/release-notes.sh vX.Y.Z`. The root `CHANGELOG.md` is upstream PocketBase's; leave it alone.
- Plans and internal notes go in a private maintainer repo, never here.
