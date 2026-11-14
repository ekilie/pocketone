# Deploy

Send your project's code to a running app from your own machine: `pb_hooks`, `pb_migrations`, `pb_public`, packages
and the production `node_modules` tree. Only changed files are uploaded, the app swaps them in and restarts when it
needs to. `pocketone dev` redeploys on every save and streams the app's logs; `pull` and `logs` work the other way.

```bash
pocketone link https://myapp.example.com   # add the remote and sign in (Remotes)
pocketone deploy                           # upload what changed
pocketone logs
```

## Install

Deploy is built into `pocketone` and **on** in both directions: every app it serves accepts deploys from its
superusers, and the `deploy`, `dev`, `pull` and `logs` commands are there to deploy to other apps. There's nothing to
install.

In your own Go build, register it before the app starts. [Remotes](../remotes/README.md) gives the commands their
remotes and logins, and [Tokens](../tokens/README.md) lets you deploy with API tokens:

```go
app := pocketbase.New()
tokens.MustRegister(app, tokens.Config{})   // optional: API tokens with the deploy scope
remotes.MustRegister(app, remotes.Config{}) // remotes, login, and /_pocker/whoami discovery
deploy.MustRegister(app, deploy.Config{})   // the deploy API + deploy/dev/pull/logs commands
app.Start()
```

The module is `github.com/pockethost/pocketone/ext/deploy`. `deploy.Config{DisableServer: true}` keeps only the
commands, for a binary that deploys elsewhere but must not accept deploys.

## Configure

There is no `pocketone.yaml` section, flag or settings screen. Defaults on the receiving app:

- **Who may deploy:** superusers. You sign in with `pocketone login` (see [Remotes](../remotes/README.md)) or use an
  API token. A token needs the `deploy` scope (or no scopes); `deploy:read` allows reading files and logs only.
- **Where files land:** next to `pb_data`, PocketBase's usual layout (`pb_hooks/`, `pb_migrations/`,
  `pb_public/`, `node_modules/` beside it).
- **Upload store:** uploaded files and past deploys are kept in `.pb_deploy/` next to `pb_data`. It's outside
  `pb_data`, so backups skip it, and it's never deployed.

On your machine, the commands use the project's remotes and saved logins from [Remotes](../remotes/README.md). In CI,
set `POCKETBASE_TOKEN` (or `POCKETBASE_TOKEN_<REMOTE>`) instead of signing in.

## Use

All commands take an optional remote name or URL; without one they use the project's default remote. Run them from
your project folder (or any folder below it).

### `deploy`

```bash
pocketone deploy                      # to the default remote
pocketone deploy prod -m "fix signup" # to the remote "prod", with a note
pocketone deploy --dry-run            # list what would be sent, send nothing
pocketone deploy --out /tmp/set       # write the files to a folder instead
```

You'll see how many files are deployed and how many are uploaded, then a line like
`Deployed <id>: 3 changed, 0 deleted, 41 unchanged. Restarted.` With nothing changed it says
`Up to date: no changes.`

| Flag | What it does |
|---|---|
| `--dry-run` | List what would be deployed; deploy nothing. |
| `--out <dir>` | Write the deploy set to a local folder instead of deploying. |
| `-i`, `--include` | Add include patterns to the default set. |
| `-e`, `--exclude` | Exclude patterns (`pb_data` is always excluded). |
| `--node-modules` | Ship the production `node_modules` tree (the default when there's a `package.json`). |
| `--no-node-modules` | Don't ship `node_modules`; leave the remote's alone. |
| `--commit <sha>` | Commit to record. Default: `$GITHUB_SHA`, `$CI_COMMIT_SHA`, or git `HEAD`. |
| `-m`, `--message` | A note to record with the deploy. |
| `-v`, `--verbose` | List every file. |

**What's sent:** files matching `pb_*` and `pb_*/**/*` except `pb_data`, plus `package.json`, `bun.lock(b)` and
`patches/**`. Dotfiles are included (`pb_public/.well-known`); `.git*` files and symlinks are skipped.

**Packages:** with a `package.json`, the production `node_modules` tree is collected the way Node resolves it (hoisted
npm and bun, pnpm's store, workspace links; never devDependencies) and shipped as plain files, so plain JS hooks can
`require` npm packages. The remote then owns its root `node_modules`: packages you no longer ship are removed there.

**Restarts:** a deploy that changes anything outside `pb_public` restarts the app about a second after it answers.
Files are swapped in while the app runs, each rename atomic. A deploy that only touches `pb_public` doesn't restart.

### `dev`

```bash
pocketone dev
```

Deploys now, then again on every change (half a second after the last save), and streams the remote's logs.
`--no-logs` turns the log stream off. It takes the same file flags as `deploy`. Stop it with Ctrl+C.

### `pull`

```bash
pocketone pull prod                              # pb_hooks, pb_migrations, pb_public, package.json
pocketone pull prod pb_hooks pb_public/index.html
pocketone pull prod --dry-run
```

Downloads files from a remote into the project. `pb_data` is only reachable on an instance that's powered off, so a
single app (always running while it answers) refuses it.

### `logs`

```bash
pocketone logs prod
pocketone logs prod -n 20
```

Replays the last 100 lines (`-n`, `--lines`, 0 to 1000), then follows. Requests show as `GET /url → 200 (1.2 ms)` and
errors are marked `ERR`. PocketBase writes its logs in batches, so lines show up a few seconds late. The stream
reconnects by itself if it drops.

### Deploy history

Every deploy is recorded in the `_deploys` collection of the receiving app: status, file counts, bytes, commit,
message, source, token name, superuser and any error. Superusers can browse it in the dashboard.

## Integrations

### Works with

| Plugin | How they fit |
|---|---|
| [Remotes](../remotes/README.md) | Gives the commands their remotes, saved logins and CI tokens. The commands find the deploy API through the remote's `/_pocker/whoami`. |
| [OAuth Server](../oauth-server/README.md) | `pocketone login` signs in through the browser when the remote runs it. |
| [Tokens](../tokens/README.md) | Deploy registers the `deploy` and `deploy:read` scopes, so you can make API tokens for CI that can only deploy or only read. |
| [Pocker](../pocker/README.md) | The same API runs for many instances with your own backend: build it with `deploy.New` (below). PocketHost does this. |
| PocketHost | A remote without `/_pocker/whoami` is taken to be a PocketHost instance: the commands use `/_ph/deploy/v1` and a PocketHost token (`POCKETHOST_TOKEN`, or the `pockethost` remote read from `.pockethostconfig`). |

### Building on Deploy

- **Go client.** `deploy.NewClient(ctx, rc)` takes a [Remotes](../remotes/README.md) client and finds its deploy API.
  `Deploy`, `Pull`, `Collect` and `Watch` are the commands' building blocks.
- **Your own host.** `deploy.New(deploy.Options{Prefix, Backend, Instances, PBRoot, DataRoot, PlatformPaths, Logs})`
  builds the API; mount `api.Wrap(next)`. `Backend` decides which instance a request is for (`Lookup(r)`), who it
  acts as (`Authenticate(r, inst)` → `Auth{UserID, Scopes, Via, TokenLabel}`, an `*AuthError`, or `ErrNoAccess` for a
  404) and records history. `Instances` is pocker's Manager (stop, swap, start). The default `FileLogs` tails each
  instance's `logs/exec.log`, which pocker writes. PocketHost composes it with its own token backend and keeps
  `/_ph/deploy/v1`.
- **Discovery.** Deploy advertises itself as the `deploy` feature of `/_pocker/whoami`, so clients find its path.

## Deploy API

`/_pocker/deploy/v1` (superusers, see Configure):

| Route | What it does |
|---|---|
| `GET /whoami` | Who the request acts as, on which instance. |
| `POST /check` | Takes a manifest, returns the hashes the server is missing. |
| `POST /blobs` | Gzipped `PHB1` bundles: `[sha256][uint32 length][bytes]`…, every hash verified, 64 MB per request. |
| `POST /commit` | Stage, swap in, record, restart. |
| `GET`/`DELETE /files` | List or delete files. |
| `GET`/`PUT /files/raw` | Download or upload one file. |
| `GET /logs` | The app's logs as server-sent events. |

The protocol is the pockethost npm CLI's, so the same client deploys to PocketOne apps and PocketHost.

## How it works

- Go module `github.com/pockethost/pocketone/ext/deploy`. [`example/main.go`](example/main.go) is an app that accepts
  deploys and can deploy (logins from [devusers](../devusers/devusers.go): admin@example.com / password).
- A deploy is a hash diff: the client sends a manifest, uploads only the missing files in bundles, then commits.
- On a single app the files are swapped in while it runs, then `app.Restart()` re-executes the process
  (`Config.Restart` overrides it; not on Windows).
- `Config.Dir` and `Config.StoreDir` move the deploy folder and the upload store; `Config.Prefix` moves the API.
- Logs on a single app come from its own `_logs` table; entries at error level and above are marked as stderr.
- The `node_modules` collector is [`nodemodules`](nodemodules/nodemodules.go), a port of the npm CLI's.

## License

MIT
