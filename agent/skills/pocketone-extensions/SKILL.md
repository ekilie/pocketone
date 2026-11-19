---
name: pocketone-extensions
description: >-
  Write or change a PocketOne extension and contribute it to github.com/pockethost/pocketone as a pull request that
  adds or edits ext/<name>/. Use when working in a pocketone checkout on an ext/<name> folder, when code uses extkit
  (Extension, Contracts(), Register, NewSection, conformance, modetest), when designing or providing a contract in
  ext/contracts (restore/v1, mail.fallback/v1, ... served at /api/_c/<id>/), when an extension's README front matter,
  CHANGELOG.md or TODO.md needs writing, or when someone wants to add a feature to PocketOne as an extension. For
  using PocketOne in an app (the pocketone binary, pocketone.yaml, deploy), use the pocketone skill; for pb_pages
  sites, the pocketpages skill.
---

# Writing a PocketOne extension

PocketOne is PocketBase plus extensions. Every feature is an **extension**: its own Go module in `ext/<name>/` of
[github.com/pockethost/pocketone](https://github.com/pockethost/pocketone), compiled into the binary. An outside
extension arrives as a **pull request that adds `ext/<name>/`**. Whoever submits it owns it: later pull requests to
it need your approval (or a maintainer's).

Words: everything in `ext/` is an **extension**, never a "plugin". **Standard extensions** ship in every `pocketone`
binary (listed in `ext/standard`); the rest are **community extensions**, which is what yours will be.

Read these before writing code (in a checkout they are `docs/contracts.md` and `docs/extensions.md`):
- [How PocketOne fits together](reference/docs/contracts/index.md): why extensions meet only through contracts.
- [Create your own extension](reference/docs/writing-an-extension.md): the full guide this skill summarizes.

## The order of work

1. **Find the contracts first** (next section). Decide what your extension provides, requires and works with before
   you write the extension.
2. **Write the extension** in `ext/<name>/`, its own Go module, touching nothing outside that folder (and
   `ext/contracts/<id path>/` for a new contract).
3. **Test it:** the conformance test, a modes test, your own tests.
4. **Write its README, CHANGELOG.md and example** in the shapes CI checks.
5. **Run what CI runs**, then open the pull request.

## Contracts come first

A contract is a named, versioned promise (`restore/v1`): a Go interface in `ext/contracts/<id path>/vN/`
(`mail.fallback/v1` lives in `ext/contracts/mail/fallback/v1/`). PocketOne turns a contract's interface into its HTTP
API at `/api/_c/<id>/<op>`, a `pocketone remote` command per operation, an MCP tool per JSON operation, Go and JS
hooks, and a client. Extensions never talk to each other directly; they meet through contracts.

Choose in this order:

1. **Use an existing contract.** Read [the contract catalogue](reference/docs/contracts/index.md). If one promises what you need,
   put it in `Requires` or `WorksWith`. If one promises what you offer, provide it: your extension then stands in for
   the current provider (a default steps aside; otherwise `providers:` in `pocketone.yaml` picks).
2. **Extend one without changing its promise:** join a chain (`mail.fallback/v1`), decorate whatever provides it
   (`Contract.Wrap`, declared in `Wraps`), add or replace `pocketone` commands (`cli.commands/v1`), or give it an
   admin screen (`Contract.UI`).
3. **Add a new contract only when none fits.** Name it for what it does (`restore/v1`), never for your extension. A
   new optional ability beside an existing contract is a small companion contract (`mail.suppression/v1` beside
   `bulkmail/v1`), not a modified copy. It goes in the same pull request, in `ext/contracts/<id path>/v1/`, with a
   README: front matter `owners` and `summary`, then `## Use`, `## Provide`, `## Extensions`, `## License` (`MIT`).
   You own the contract you create. In the pull request, name the contracts you checked and why none fits.

A published `vN` never changes what it promises. A changed promise is a new `vN+1` beside it, reviewed with the
contract's owners.

After adding or changing a contract interface, from the repo root:

```bash
(cd ext/contracts && go generate ./...)
go run ./.github/extensions check
```

## The rules every extension follows

- **No routes of its own.** Every API a client can call is a contract operation under `/api/_c/<id>/`. Only paths a
  standard fixes stay at the root (`/.well-known/*`, `/mcp`), and those belong to existing extensions.
- **Never import another extension's module** (tests and examples may). Anything shared (helpers, scopes, a client)
  belongs in the contract package.
- **Declare by contract id, never by extension name**, in `Contracts()`: `Implements`, `Defaults`, `Wraps`,
  `Requires`, `WorksWith`.
- **Provide in `Register`** (`Provide` / `ProvideDefault`); **look contracts up only later** (`Of`, `Must`, in hooks
  or at serve time). A lookup inside `Register` panics.
- **Collections you add** are named `_<name>_<table>`, hyphens in the name as underscores (`_agent_chat_messages`);
  never change PocketBase's own collections.
- **Your schema ships as your own migrations** (a `Migrations() []extkit.Migration` method; extkit applies them at
  bootstrap and records them in `_extension_migrations`), never copied into an app's `pb_migrations`.
- **Files you can rebuild** go in `extkit.CacheDir(app, name)` (`pb_cache/<name>/`): never backed up or deployed, and
  deletable at any time.
- **Dependencies stay in your `go.mod`**, never core's. A module that reaches another `ext/` module (a contract
  package, `extkit`) adds a `replace` for it.
- **Environment variables** your extension reads are `P1_<NAME>` (`extkit.EnvVar("TOKEN")` is `P1_TOKEN`). `PB_` is
  PocketBase's.
- **Messages that name a command** use `extkit.Command` (default `pocketone`), never a literal.
- **Standalone first:** where it makes sense, it works on one app, and does more on a multisite host.
- **MIT.** An extension under another license lives in its author's own repository, not here.

## Don't change PocketBase's core

Core is every path outside `ext/`, `site/`, `examples/pocketone/`, `docs/` and the repo's own docs and CI files. Most
extensions need no core change: use `OnServe`, the record and request hooks, the JS engine's extension methods
(`OnInit`, ...), `OnFilesystemCreate`, `BaseAppConfig.AppMigrations` / `OnLog` / `DBConnect`, `UIExtensions`.

If you truly need something core doesn't offer, add the **smallest generic hook** (a nil-checked function field, a
list callers append to, or an event), named for what it allows, not for your extension, with upstream behavior
unchanged when nothing uses it. Mark every changed line `// PocketOne:` with a few words on why, put it in its own
commit with a test, ahead of the extension, and say in the pull request why it can't live in `ext/`. A maintainer
reviews and audits every core change. Ask the user before making one; never change core casually.

## The extension

```
ext/<name>/
  go.mod               module github.com/pockethost/pocketone/ext/<name>
  README.md            its docs and its page on pocketone.dev
  CHANGELOG.md         required
  TODO.md              optional: work that waits on something else existing first
  <name>.go, <name>_test.go
  conformance_test.go  required
  modes_test.go        the feature on the real binary, in every mode it runs in
  example/main.go      required: the smallest program that shows it working
  ui/                  optional: admin screens (embedded main.js)
  site/                optional: more docs pages, served under its pocketone.dev page
```

```go
type Extension struct{ cfg Config }

func New(cfg Config) *Extension { return &Extension{cfg: cfg} }

func (e *Extension) Name() string { return "hello" } // its pocketone.yaml key, `_hello` collections, log lines

func (e *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{
		Implements: extkit.IDs("deploy/v1"),
		Defaults:   extkit.IDs("deploy/v1"), // steps aside if another extension provides it
		WorksWith:  extkit.IDs("tokens/v1"),
	}
}

func (e *Extension) Register(app core.App, cfg extkit.Config) error {
	c := e.cfg
	if err := cfg.Decode(&c); err != nil { // this app's settings over the defaults
		return err
	}
	deployv1.Contract.ProvideDefault(app, myDeployer{})
	return nil
}
```

- **Settings in `pocketone.yaml`:** `extkit.NewSection(name, defaults, doc)`, with a `doc:"…"` struct tag on each
  key (it feeds `pocketone config --json` and the key table on your page). An empty section means all defaults.
- **Settings admins change in the dashboard** go through `settings/v1` (`settingsv1.Add`).
- **Per-app state** in `app.Store()`; shared resources close through `extkit.Closer` (`Close(ctx) error`).
- **Per-instance keys** (bucket prefixes, rate-limit keys) from `extkit.AppID(app)`; never build paths from request
  input. One extension value serves many apps on a multisite host.
- **A JS API** for `pb_hooks`: `BindJS(app, rt)` sets globals, and a `TypeScript()` method declares their types.
- **Admin screens:** append `core.UIExtension{Name, FS}` to `se.UIExtensions` in `OnServe`.
- **MCP tools** are named `<name>_<verb>_<noun>` (`pb_` is reserved).
- **Host extensions** (gates, TLS, middleware, runtimes, instance sources) are `multisite.Extension`s with `Name()`
  and `RegisterHost(h)`.
- To try it in a binary: `set.Add(myext.New(...))` on the standard set, as in `ext/standard/example/main.go`.
- Good models to copy: `ext/bulkmail/extension.go` (an app extension), `ext/pitr/README.md` (the README).

## Tests

```go
func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return myext.New(myext.Config{}) })
}
```

The conformance test checks that the extension registers, migrates, serves and shuts down with no goroutines left
behind, has only contract routes, names its collections right, and serves several apps from one value. Pass
`conformance.Options{Deps: ...}` for extensions it requires and `Setup` when it adds MCP tools.

`modes_test.go` runs the same test body on the real `pocketone` binary in each mode with `ext/extkit/modetest`:
`modetest.AllApp` (one app, and a multisite host) for an app extension; Host and/or Fleet for a host extension.

`example/main.go` calls `devusers.Register(app)` right after creating the app, so a fresh data dir has superuser
`admin@example.com` and user `user@example.com`, both password `password`. Never in a production build.

Tests that need an external tool (MinIO, Docker) skip themselves when it isn't installed.

## README, changelog, TODO

The README is also the extension's page on pocketone.dev. Front matter, then five sections in this order:

```markdown
---
owners: ["@your-github-handle"]
version: 0.1.0
title: "My Extension"
group: services
summary: "One line: what it does for someone running an app."
---
```

- `group` is one of `binary`, `many`, `services`, `saas`, `authors` (from `site/extensions.json`).
- `## Install`: how to get it (the lines to add to a Go build).
- `## Configure`: its `pocketone.yaml` section, flags and settings screens.
- `## Use`: day to day: screens, commands.
- `## Integrations`: the extensions and contracts it works with, and how others can build on it.
- `## Example`: a sentence, then the whole `example/main.go` in a `go` block, exactly as it is
  (`go run ./.github/extensions examples` copies it in; CI fails when they differ).

Write Install, Configure and Use for someone running one app, not for a Go developer. Extra sections (how it works,
limits) come after. The README ends with `## License` and the line `MIT`.

`CHANGELOG.md`: every user-visible change adds one line under `## Unreleased`, grouped under `### Upgrading`
(steps for anything breaking), `### Added`, `### Changed` or `### Fixed`. **Don't raise `version:` yourself:** each
PocketOne release does it for every extension with unreleased lines.

`TODO.md` (optional): only work that waits for something else to exist first, each item under a `**Waits on:**` line
naming what it waits for. It isn't published.

## Before you open the pull request

From the repository root:

```bash
go build -o /dev/null ./...                    # a plain `go build` writes a binary into the tree
go run ./.github/extensions test ext/<name>    # everything CI runs on your pull request; ends with `ok`
cd site/sync && go run . --validate            # your README builds into a page
```

- **Commit only your folder** (and a new contract's folder). Stage files by name; never commit binaries.
- **Never commit generated files:** site pages, nav, search index, `llms.txt`, contract indexes and the skills'
  `reference/` folders are rebuilt on `main` after your pull request merges. To preview your page, run
  `cd site/sync && go run .`, then discard what it writes outside your folder.
- **Who approves:** a maintainer for a new extension, a new contract, or anything outside community extensions; one
  of its owners for a change to an existing community extension or contract. Nothing merges until CI passes.
  `go run ./.github/extensions approvers origin/main HEAD` lists who may approve each part of a change.

## Reference index

Plain-Markdown copies of pocketone.dev, regenerated by `site/sync`.

- [Create your own extension](reference/docs/writing-an-extension.md): the full guide
- Contracts: [how they fit together](reference/docs/how-pocketone-fits-together.md),
  [the catalogue](reference/docs/contracts/index.md) (what each promises, who provides and uses it), each at
  `reference/docs/contracts/<id>/index.md`
- Existing extensions: [the catalogue](reference/docs/extensions/index.md), each README at
  `reference/docs/extensions/<name>/index.md`; [pitr](reference/docs/extensions/pitr/index.md) is the model README
- Config: [pocketone.yaml reference](reference/docs/config-reference.md)
- Using PocketOne in an app: the `pocketone` skill. Building `pb_pages` sites: the `pocketpages` skill.
