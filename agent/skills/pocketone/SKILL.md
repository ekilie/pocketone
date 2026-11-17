---
name: pocketone
description: >-
  Work with PocketOne, PocketHost's community fork of PocketBase with extensions in ext/. Use when a project has a
  pocketone.yaml (or its .env), runs the `pocketone` binary (serve, superuser, pages create, login, link, deploy, dev,
  pull, logs, upgrade, config), deploys with `pocketone deploy`, or when writing or changing a PocketOne extension
  (ext/<name>, extkit, contracts under /api/_c/, README front matter, site/sync). Also use when a PocketBase project
  wants something PocketOne adds: point-in-time restore (PITR) or restore to a moment from backups, a dev mail inbox,
  a mail relay or bulk mail, many sites behind one listener (multisite, Pocker, Docker), long-lived API tokens and an
  OAuth server, an MCP endpoint for agents, automatic TLS, or signed S3 download redirects.
---

# PocketOne

PocketOne is a community fork of PocketBase, based on upstream v0.41.1, maintained by PocketHost. It is not affiliated
with or endorsed by the PocketBase project. Its changes to PocketBase's own code ("core") are kept to hooks and a few
bug fixes; every feature is an **extension** in `ext/<name>/`, each its own Go module. The stock binary is
`pocketone` (built from `examples/pocketone`).

Words: everything in `ext/` is an **extension**, never a "plugin". **Standard extensions** ship in every `pocketone`
binary; the rest are **community extensions**. "Plugin" means only upstream's `plugins/` (jsvm, migratecmd, ghupdate)
and PocketPages' own plugin system (`+config.js` `plugins`).

This skill has two parts: [using PocketOne in an app](#part-1-using-pocketone-in-an-app) and
[writing or changing an extension](#part-2-writing-or-changing-an-extension). For building a `pb_pages` app (routes,
EJS templates, layouts, PocketPages plugins), use the **`pocketpages`** skill; this one only covers how PocketOne
serves and deploys it.

## Part 1: Using PocketOne in an app

### Install

```bash
brew install pockethost/tap/pocketone     # macOS or Linux (formula in pockethost/homebrew-tap)
# or: the zip for your platform from https://github.com/pockethost/pocketone/releases, on your PATH
# or from source:
git clone https://github.com/pockethost/pocketone.git
cd pocketone/examples/pocketone
go build -o pocketone .
```

`pocketone --version` prints the PocketOne version and the PocketBase version under it, e.g. `0.0.2 (PocketBase 0.41.1)`.

### First run

Create the superuser before the first `serve` of a fresh data dir; otherwise `serve` opens the superuser install page
in a browser:

```bash
pocketone superuser upsert admin@example.com password --dir ./pb_data
pocketone serve --dir ./pb_data          # http://127.0.0.1:8090, dashboard at /_/
pocketone serve --dev --dir ./pb_data    # dev mode
# a server-rendered site from a starter kit:
pocketone pages starters                 # auth, daisyui, daisyui-docs, datastar, htmx, minimal (default), mvp
pocketone pages create myapp --starter htmx
cd myapp && pocketone serve --dev --dir ./pb_data
```

PocketOne reads the same `pb_data`, `pb_hooks`, `pb_migrations` and `pb_public` folders as PocketBase. To move an
existing app, back up `pb_data`, then point `--dir` at it.

### How it differs from stock PocketBase

Same dashboard, collections, rules, REST and realtime APIs, JS hooks and migrations; PocketBase's docs still apply to
those. What changes:

- **PITR is on by default.** The database is replicated with Litestream into `pb_data/litestream/`; restore from
  Settings → Backups, `pocketone pitr restore`, or `pocketone remote restore to --at -2h`. `--pitr=false` turns it off.
- **Dev mode catches mail.** With `serve --dev` and no SMTP, mail goes to an inbox at http://127.0.0.1:8025 instead
  of `sendmail` (Mail Trap).
- **`pb_pages` is served** when the folder exists next to `pb_data` (`--pagesDir` picks another).
- **A config file**, `pocketone.yaml`, with secrets in a `.env` beside it. Without it, nothing changes.
- **Remote commands**: `login`, `link`, `remote`, `deploy`, `dev`, `pull`, `logs`, `restart`.
- **Agents and tokens**: an MCP endpoint at `/mcp`, long-lived API tokens (`pbt_…`) with scopes, and an OAuth server
  used by `pocketone login` and MCP sign-in.
- **`pocketone upgrade`** replaces the executable and keeps the old one as `pocketone-<version>`.
- **Extension APIs are contracts** under `/api/_c/<id>/` (see [Contracts](#contracts)). `/mcp` and `/.well-known/*`
  are the only extension paths at the root. Responses carry `X-Powered-By: PocketOne <version>`.
- **Many instances**: a `multisite` section in `pocketone.yaml` makes `pocketone serve` a host for many instances
  (in-process with Pocker, in containers with Docker, or both).

### Commands

The `pocketone` binary has PocketBase's commands (`serve`, `superuser`, `migrate`) and flags, plus these. Each flag is
on the commands that read it, so put it after the command name; only `--config` and `--env-file` are on every command.
Run `pocketone <command> --help` for the full list.

| Command | What it does |
|---|---|
| `serve [domains]` | Starts the server. `--http`, `--https`, `--origins`, `--publicDir`, `--pagesDir`, `--pitr`, `--autotls`, and the app flags (`--dir`, `--dev`, `--hooksDir`, `--migrationsDir`, `--jsvm`). |
| `superuser create\|upsert\|update\|delete\|otp\|ips` | Manage superusers. |
| `pages create <name> [--starter kit]`, `pages starters` | A new PocketPages folder from a starter kit. |
| `config [--json]`, `config check` | List every `pocketone.yaml` section with defaults; check a file and its `$VAR`s. |
| `link <url> [token]` | Add a remote (default name `default`, `--name`) and sign in. |
| `login [remote]`, `logout [remote]` | Sign in (browser OAuth, `--device`, `--token`, `--token-stdin`; never a password); forget the login. |
| `remote add <name> <url> [--default]`, `list`, `default`, `rm`, `whoami`, `status`, `logs`, `restart` | Manage remotes; inspect an app. |
| `remote <contract> <op>` | Every contract operation an app offers, e.g. `remote restore points`, `remote restore to --at -2h`, `remote settings`, `remote bulkmail`, `remote mail suppression`. |
| `deploy [remote]` | Upload changed `pb_*` files (not `pb_data`), `package.json`, `bun.lock(b)`, `patches/` and production `node_modules`. `--restart`, `--dry-run`, `--out <dir>`, `-m`, `-i`/`-e`, `--commit`, `--no-node-modules`. |
| `dev [remote]` | Two-way sync with a remote and stream its logs. `--prefer local\|remote`, `--push-only`, `--poll`, `--no-logs`. Type `r` and Enter to restart. |
| `pull [remote] [paths...]` | Download files from a remote (`--dry-run`). |
| `logs [remote] [-n lines]` | Stream a remote's app logs. |
| `restart [remote]` | Restart a remote so new migrations and packages apply. |
| `pitr status`, `pitr restore --at <time> [--to file]` | Local point-in-time restore (in place runs on the next start). |
| `token create\|list\|revoke\|scopes` | API tokens (`pbt_…`). |
| `oauth clients` | OAuth server clients. |
| `template list`, `template eject [set] [--force]` | Extensions' page templates (`oauth`: the sign-in screens): list them, or copy them to `pb_templates/<set>/` to edit; eject asks before overwriting your changes. |
| `upgrade [-y] [--backup]` | Move to the latest release (prereleases included; asks before stable → prerelease). |
| `docker seed\|update` | Release binaries for the Docker runtime (host mode only). |

Root flags `--config <path>` and `--env-file <path>` pick the config file and its `.env`.

### Deploy workflow

```bash
pocketone link https://myapp.example.com      # add remote "default" and sign in
pocketone deploy -m "fix signup"              # changed files only; hooks, pages, public go live at once
pocketone deploy --restart                    # restart if migrations or packages changed (use in CI)
pocketone dev                                 # live two-way sync while you work
pocketone logs -n 200
```

- A command with no remote uses the default remote, else the only one, else the one called `default`. Any command
  also takes a URL instead of a name.
- Deploy never restarts on its own. Migrations and packages apply when the app starts: `--restart`,
  `pocketone restart`, or `r` in `dev`.
- `dev` keeps state in `.pocketone/` and puts the remote's side of a conflict in `.pocketone/conflicts/`; list files
  to leave out in `.pocketoneignore`.
- Login is the browser OAuth grant (`--device` on headless machines) or an API token; it never takes a password.

### pocketone.yaml

- Found in the working directory, then next to the executable; `--config` picks another. The `.env` is found beside
  it; `--env-file` picks another. Relative paths are relative to the file's folder.
- Each top-level key is a section owned by one extension. An unknown key, a duplicate section, a wrong type or an
  unset `$VAR` stops startup. An empty section means all defaults.
- **Flags win** over the file; the file only changes defaults.
- `$VAR` / `${VAR}` reads the `.env` first, then the environment (`$$` is a literal `$`). The `.env` is never put in
  the process environment, so JS hooks (`$os.getenv`) don't see it unless the name is in `app.env_allow`.

```yaml
server:                          # what serve reads
  http: 127.0.0.1:8090
  origins: ["*"]
  public_dir: ./pb_public
app:                             # what every command that opens the app reads
  dir: ./pb_data
  hooks_dir: ./pb_hooks
  env_allow: [STRIPE_KEY]
pitr:
  retention: 168h
autotls:
  auto: true
  email: ops@example.com
  dns:
    provider: cloudflare
    api_token: $CF_API_TOKEN      # from the .env
providers:                        # pick a provider when several extensions provide a contract
  restore/v1: pitr
```

Sections in the stock binary: `app`, `server`, `autotls`, `pitr`, `s3redirect`, `providers`, and for hosts
`multisite`, `instances`, `pocker`, `docker`, `firewall`, `rate_limits`, `edges`, `mothership`, `lifecycle`. Run
`pocketone config` to see them with defaults, and read
[reference/docs/config-reference.md](reference/docs/config-reference.md) for every key.

### Extensions

From each README's front matter. "In `pocketone`": **yes** = on in the stock binary; **host** = used when
`pocketone.yaml` has a `multisite` section; **Go build** = add it to your own `main.go`; **planned** = README only,
no code yet (version 0.0.0).

| Extension | In `pocketone` | What it's for | Reference |
|---|---|---|---|
| **Standard extensions** | | | |
| PITR | yes | Point-in-time restore of your database with Litestream. On by default. | [pitr](reference/docs/extensions/pitr/index.md) |
| Settings | yes | One API for the settings an app's admins change for its extensions. | [settings](reference/docs/extensions/settings/index.md) |
| Backups | yes | Restore an app to a moment from its own backups: the latest one before it. | [backups](reference/docs/extensions/backups/index.md) |
| Auto TLS | yes (`--autotls`) | Certificates on demand and wildcards through ACME. | [autotls](reference/docs/extensions/autotls/index.md) |
| Bulk Mail | yes | Campaigns to your app's own users, with audiences from a collection, a view or SQL. | [bulkmail](reference/docs/extensions/bulkmail/index.md) |
| Pages | yes | Server-rendered pages from a pb_pages folder (formerly PocketPages). | [pages](reference/docs/extensions/pages/index.md) |
| Vacuum | planned | Scheduled SQLite VACUUM. | [vacuum](reference/docs/extensions/vacuum/index.md) |
| MCP | yes | An MCP endpoint at /mcp so agents can manage your app and call its tools. | [mcp](reference/docs/extensions/mcp/index.md) |
| S3 Redirect | yes | File downloads as short-lived signed S3 redirects. | [s3redirect](reference/docs/extensions/s3redirect/index.md) |
| Tokens | yes | Long-lived API tokens with scopes (and the OAuth server). | [tokens](reference/docs/extensions/tokens/index.md), [OAuth](reference/docs/extensions/tokens/oauth.md) |
| Deploy | yes | pocketone deploy, dev, pull and logs, and the deploy API. | [deploy](reference/docs/extensions/deploy/index.md) |
| Remotes | yes | Named remotes and saved logins for the pocketone command. | [remotes](reference/docs/extensions/remotes/index.md) |
| Upgrade | yes | pocketone upgrade to the latest release, keeping the old executable. | [upgrade](reference/docs/extensions/upgrade/index.md) |
| Config | yes | The pocketone.yaml config file, with secrets in a .env beside it. | [config](reference/docs/extensions/config/index.md) |
| JSVM | yes | The JS engine for pb_hooks and pb_migrations: one QuickJS VM per app, real globals, async handlers, hard limits. | [jsvm](reference/docs/extensions/jsvm/index.md) |
| **Run many instances** | | | |
| Pocker | host | Hundreds of PocketBase instances in one Go process, routed by hostname. | [pocker](reference/docs/extensions/pocker/index.md), [guide](reference/docs/extensions/pocker/pocker.md) |
| Multisite | host | Many instances behind one listener, routed by hostname, run by one provider. | [multisite](reference/docs/extensions/multisite/index.md) |
| Docker | host | A host provider that runs each instance in its own container. | [docker](reference/docs/extensions/docker/index.md) |
| Sandbox | host (via Pocker) | Confines untrusted JS hooks, SQL and outbound network. | [sandbox](reference/docs/extensions/sandbox/index.md) |
| Firewall | host | Peer allow-lists, trusted proxies and rate limits for a host. | [firewall](reference/docs/extensions/firewall/index.md) |
| Edge | host | Forwards each request to the edge that owns the instance. | [edge](reference/docs/extensions/edge/index.md) |
| Mothership | host | A control plane and its edges, across regions. | [mothership](reference/docs/extensions/mothership/index.md) |
| Schedules | planned | Scheduled webhooks that wake sleeping instances. | [schedules](reference/docs/extensions/schedules/index.md) |
| Stats | host | Measures disk use and last request for each instance a host serves. | [stats](reference/docs/extensions/stats/index.md) |
| **Services for your instances** | | | |
| Mail Relay | Go build | Outgoing mail for instances without SMTP, through your relay, with limits. | [mailrelay](reference/docs/extensions/mailrelay/index.md) |
| S3 Fallback | Go build | A default bucket for uploads and backups when an instance has no S3 of its own. | [s3-fallback](reference/docs/extensions/s3-fallback/index.md) |
| Server Timing | host | Server-Timing headers for each step of a request. | [servertiming](reference/docs/extensions/servertiming/index.md) |
| **Sell it as a service** | | | |
| Multitenant | planned | Accounts that own instances: quotas, owner logins, tokens and an API. | [tenants](reference/docs/extensions/tenants/index.md) |
| Multitenant Billing | planned | Plans, quotas and the billing provider hook for Multitenant. | [tenant-billing](reference/docs/extensions/tenant-billing/index.md) |
| Lemon Squeezy Adapter | planned | Lemon Squeezy as the billing provider. | [tenant-billing-ls](reference/docs/extensions/tenant-billing-ls/index.md) |
| **For extension authors** | | | |
| Extension Kit | Go library | The extension contract: one extension works on a single app and under pocker. | `ext/extkit/extkit.go` (no page) |
| Standard Set | yes | The extensions every PocketOne app has, as one set a host configures. | [standard](reference/docs/extensions/standard/index.md) |
| Contracts | Go library | The versioned interfaces extensions use to work together, and their generator. | [all contracts](reference/docs/contracts/all.md) |
| Mail Trap | yes (dev mode) | Dev mode: mail from an app without SMTP lands in a built-in inbox. | [mailtrap](reference/docs/extensions/mailtrap/index.md) |
| Dev Users | Go build | Local testing only: a superuser and a user with password "password". | `ext/devusers/devusers.go` (no page) |
| Bundle | Go build | An app's hooks, migrations and public files embedded in the binary. | [bundle](reference/docs/extensions/bundle/index.md) |

To turn a standard extension off, set `enabled: false` in its `pocketone.yaml` section; extensions that require it
turn off too, and the startup lines say why.

## Part 2: Writing or changing an extension

Read [reference/docs/contracts/index.md](reference/docs/contracts/index.md) (the idea) and
[reference/docs/writing-an-extension.md](reference/docs/writing-an-extension.md) (the code) first. In the repo these
are `docs/contracts.md` and `docs/extensions.md`; `AGENTS.md` has the rules.

### Never change PocketBase core casually

Core is every path outside `ext/`, `site/`, `examples/pocketone/`, `docs/`, `README.md`, `CONTRIBUTING.md`,
`AGENTS.md`, `CLAUDE.md`, `.github/`, `.claude/` (the exact list is in `AGENTS.md`). Put features in `ext/`. Prefer
seams that exist: `OnServe`, record and request hooks, jsvm `Config.Extensions` (`OnInit`, `OnCall`, ...),
`OnFilesystemCreate`, `BaseAppConfig.AppMigrations` / `OnLog` / `DBConnect`, `UIExtensions`.

If a core change is truly needed: the smallest generic hook, upstream behavior unchanged when unused, every changed
line marked `// PocketOne:`, its own commit with a test. CI (`.github/workflows/core-audit.yml`) fails a commit that
changes core without a `Core-Audit:` trailer from an independent reviewer (a fresh agent with no context, or a human):

```bash
.github/core-audit/core-diff.sh origin/main HEAD    # the diff to hand the reviewer
# trailer, last line of the commit message:
# Core-Audit: pass - <what the core change is and why it can't live in ext/>
```

A new hook also needs its `core-hook/<name>` branch and a row in the README's "Changes we made to PocketBase" table.
Don't do this unasked; stop and say a core change is needed.

### Folder layout

```
ext/<name>/
  go.mod                module github.com/pockethost/pocketone/ext/<name>
  README.md             docs and its pocketone.dev page
  <name>.go, <name>_test.go
  conformance_test.go   required
  example/main.go       required: minimal program that builds and shows the feature
  ui/                   optional admin screens (main.js)
```

Heavy dependencies go in the extension's `go.mod`, never core's. A module that reaches another `ext/` module adds a
`replace` for it. Builds use PocketBase's module path, so a final binary's `go.mod` needs
`replace ganigeorgiev.com/pocketbase/pocketbase => github.com/pockethost/pocketone <commit or version>`.

### extkit

```go
type Extension struct{ cfg Config }

func New(cfg Config) *Extension { return &Extension{cfg: cfg} }

func (e *Extension) Name() string { return "hello" } // config key, `_hello` collections, logs

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

// optional: a $hello global in pb_hooks (js is ext/extkit/js)
func (e *Extension) BindJS(app core.App, rt js.Runtime) { _ = rt.Set("$hello", map[string]any{"hi": func() string { return "hi" }}) }
```

- Add it to the standard set: `set.Add(myext.New(...))`. Without the set: `extkit.MustRegister(app, exts...)` and
  `extkit.BindJS(app, rt, exts...)` in jsvm's `OnInit`.
- Process-wide settings: a `pocketone.yaml` section with `extkit.NewSection(name, defaults, doc)`; describe each key
  with a `doc:"…"` struct tag (it feeds `pocketone config --json` and the site's key table).
- Per-app state in `app.Store()`; shared resources close through `extkit.Closer` (`Close(ctx) error`).
- Per-instance keys (bucket prefix, rate-limit key) from `extkit.AppID(app)`; never build paths from request input.
- Messages that name a command use `extkit.Command` (default `pocketone`), never a literal.
- Admin screens: append `core.UIExtension{Name, FS}` to `se.UIExtensions` in `OnServe`; settings go through
  `settings/v1` (`settingsv1.Add`).
- Collections an extension adds are named `_<name>` or `_<name>_…`.
- Host extensions (gates, TLS, middleware, runtimes, instance sources) are `multisite.Extension`s with `Name()` and
  `RegisterHost(h)`.
- Reference implementation: `ext/bulkmail/extension.go`; complete program: `ext/standard/example/main.go`.

### Contracts

A contract is a named, versioned promise (`restore/v1`): a Go interface in `ext/contracts/<id path>/vN/`
(`mail.fallback/v1` → `ext/contracts/mail/fallback/v1/`). A wire contract's interface generates its HTTP API at
`/api/_c/<id>/<op>`, a `pocketone remote` command per operation, an MCP tool per JSON operation, Go and JS action
hooks, and a client. All callers pass the same scope check and hooks.

Rules:

1. **Use an existing contract first.** Check [reference/docs/contracts/all.md](reference/docs/contracts/all.md). Put
   what you need in `Requires` or `WorksWith`; if one already promises what you offer, provide it.
2. **Extend without changing the promise:** join a chain (`mail.fallback/v1`), decorate with `Contract.Wrap`
   (declared in `Wraps`), add or replace commands through `cli.commands/v1`, or add a screen with `Contract.UI`.
   A changed promise is a new `vN+1` beside `vN`; never change a published `vN`.
3. **A new contract only when none fits**, named for what it does, never for your extension. Name the contracts you
   checked in the commit message. Its README has front matter `owners` and `summary`, then `## Use`, `## Provide`,
   `## Extensions` (generated), `## License` (MIT). Whoever creates a contract owns it.

- Extensions register **no routes of their own** and **never import another extension's module** (tests and examples
  may). Shared helpers belong in the contract package.
- Declare by contract id in `Contracts()` (`Implements`, `Defaults`, `Wraps`, `Requires`, `WorksWith`); provide in
  `Register` (`Provide` / `ProvideDefault`); look up only after registration (`Of`, `Must` in hooks or at serve
  time). A lookup inside `Register` panics.
- Two non-default providers of a single-provider contract fail startup until `providers:` in `pocketone.yaml` picks.
- `pb_hooks` can take part: `onContracts((e) => { e.provide(...); e.require(...); e.worksWith(...); e.next() })`.
- After changing a contract interface:

```bash
cd ext/contracts && go generate ./...
go run ./.github/extensions contracts    # from the repo root: catalogue + each contract's Extensions section
go run ./.github/extensions check
```

### Tests and examples

```go
func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return myext.New(myext.Config{}) })
}
```

It checks register, migrate, serve and shutdown with no leaked goroutines, contract-only routes, collection names,
and one value serving several apps. Pass `conformance.Options{Deps: ...}` for required extensions and `Setup` for MCP
tools (named `<name>_<verb>_<noun>`). Run with `go test -race`. `example/main.go` calls `devusers.Register(app)` so a
fresh data dir has `admin@example.com` / `user@example.com`, password `password`; never in a production build.

### README shape (CI checks it)

```markdown
---
owners: ["@your-github-handle"]
version: 0.1.0
---
```

Then five sections, in this order: `## Install`, `## Configure`, `## Use`, `## Integrations`, `## Example`. Write the
first three for someone running one app. Example: a sentence, then the whole `example/main.go` in a `go` block,
copied in by `go run ./.github/extensions examples` (CI fails when they differ). Extra sections (How it works,
limits) come after; the README ends with `## License` and the line `MIT`. Every merge that changes an extension's code
raises its `version`.

### Site and docs: same change, every time

pocketone.dev (`site/`) copies extension READMEs, `docs/extensions.md` and `docs/contracts.md`. When an extension
changes, in the same change:

1. Update its README (all five sections, Example matching `example/main.go`). Its front matter is its catalogue
   entry: `owners`, `version`, `title`, `group` (an id in `site/extensions.json`) and a one-line `summary`. A new
   extension lists itself there; nothing outside its folder names it.
2. Add a line under `## Unreleased` in the extension's own `ext/<name>/CHANGELOG.md` for a user-visible change
   (the root `CHANGELOG.md` is for core and the binary; `CHANGELOG.pocketbase.md` is upstream's). Don't raise
   `version:`: the release step does.
3. Don't edit generated pages in `site/`; edit the source. Generated files (pages, nav, search index, these reference
   files, contract indexes) are rebuilt on `main` by `.github/regenerate.sh`, so committing them is optional; run
   `cd site/sync && go run .` to preview.
4. Run `go run ./.github/extensions test ext/<name>`: everything CI runs on a pull request.

CI checks structure, not prose, so reread the README against the code.

### Checks before you finish

```bash
go build -o /dev/null ./...                          # plain `go build` writes a binary into the tree
go run ./.github/extensions test                     # what CI runs on a pull request: rules, build, vet, -race tests, site
```

Stage files by name; never commit binaries.

## Reference index

Plain-Markdown copies of pocketone.dev, regenerated by `site/sync`.

- Start: [overview](reference/docs/overview.md), [installation](reference/docs/installation.md),
  [what PocketOne adds](reference/docs/what-pocketone-adds.md), [PocketBase docs pointer](reference/docs/pocketbase.md)
- Config: [pocketone.yaml reference](reference/docs/config-reference.md)
- Extensions: [index](reference/docs/extensions/index.md), each at `reference/docs/extensions/<name>/index.md`;
  guides [Tokens OAuth](reference/docs/extensions/tokens/oauth.md), [Pocker](reference/docs/extensions/pocker/pocker.md),
  [Pages spec](reference/docs/extensions/pages/spec.md)
- Writing extensions: [writing an extension](reference/docs/writing-an-extension.md)
- Contracts: [how they fit together](reference/docs/contracts/index.md), [all contracts](reference/docs/contracts/all.md),
  each at `reference/docs/contracts/<id>/index.md`:
  [bulkmail/v1](reference/docs/contracts/bulkmail/v1/index.md), [cli.commands/v1](reference/docs/contracts/cli.commands/v1/index.md),
  [deploy/v1](reference/docs/contracts/deploy/v1/index.md), [gate/v1](reference/docs/contracts/gate/v1/index.md),
  [http.wrap/v1](reference/docs/contracts/http.wrap/v1/index.md), [instance.source/v1](reference/docs/contracts/instance.source/v1/index.md),
  [mail.fallback/v1](reference/docs/contracts/mail.fallback/v1/index.md), [mail.feedback/v1](reference/docs/contracts/mail.feedback/v1/index.md),
  [mail.suppression/v1](reference/docs/contracts/mail.suppression/v1/index.md), [mcp/v1](reference/docs/contracts/mcp/v1/index.md),
  [oauth/v1](reference/docs/contracts/oauth/v1/index.md), [remote/v1](reference/docs/contracts/remote/v1/index.md),
  [restore/v1](reference/docs/contracts/restore/v1/index.md), [runtime/v1](reference/docs/contracts/runtime/v1/index.md),
  [settings/v1](reference/docs/contracts/settings/v1/index.md), [tls/v1](reference/docs/contracts/tls/v1/index.md),
  [tokens/v1](reference/docs/contracts/tokens/v1/index.md)
- Building `pb_pages` apps: the `pocketpages` skill.
