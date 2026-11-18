Source: https://pocketone.dev/docs/features/deploy

# Deploy and live sync

> **PocketBase:** you copy files to the server yourself and restart it. **PocketOne:** `pocketone deploy` uploads only what changed, and `pocketone dev` keeps a folder in step both ways.

Write your hooks, migrations and pages on your own machine, then send them to your running app with one command. Only
the files that changed are uploaded, and the app swaps them in while it keeps serving.

This is on for every PocketOne app. Its superusers can deploy to it, and the `pocketone` command can deploy to any
PocketOne app. You don't need to set anything up.

## Deploy from your terminal

Run these from your project folder, the one that holds `pb_hooks`, `pb_pages` and the rest:

```bash
pocketone link https://myapp.example.com   # add the app as a remote and sign in, once
pocketone deploy                           # upload what changed
```

`link` signs you in through your browser when the app offers it, or asks for an API token. When the deploy is done you
see a line like `Deployed <id>: 3 changed, 0 deleted, 41 unchanged.` With nothing to send, it says
`Up to date: no changes.`

A few more ways to run it:

```bash
pocketone deploy prod -m "fix signup"   # to the remote named prod, with a note
pocketone deploy --dry-run              # list what would be sent, send nothing
pocketone deploy --restart              # restart afterwards if migrations or packages changed
pocketone restart                       # restart later instead
```

Every command takes a remote's name or a URL. Without one, it uses the project's default remote.
[Remote CLI](https://pocketone.dev/docs/features/remote) covers adding more remotes and signing in.

## When changes take effect

- **Right away:** `pb_hooks`, `pb_pages` and `pb_public`. Hooks reload by themselves, and pages and public files are
  read fresh on the next request.
- **After a restart:** migrations and npm packages. The deploy tells you `Migrations or packages changed` and leaves
  the restart to you: pass `--restart`, or run `pocketone restart`. Your first deploy that creates `pb_hooks` needs a
  restart too.

A deploy never restarts the app on its own.

## Work live with `dev`

```bash
pocketone dev
```

`dev` keeps your folder and the app in step both ways, and streams the app's logs. Your edits go up half a second after
you save. Changes made on the app, such as the migrations the dashboard writes when you edit a collection, come down
within a few seconds. After you upload a migration or new packages, type `r` and Enter to restart the app. Stop with
Ctrl+C.

If a file changed on both sides, your copy wins and the app's copy is saved in `.pocketone/conflicts/`. `dev` keeps
its state in `.pocketone/`, so add that folder to `.gitignore`. To leave files out, list gitignore-style patterns in
`.pocketoneignore`.

## Pull files and read logs

```bash
pocketone pull                         # download pb_hooks, pb_migrations, pb_pages, pb_public, pb_templates, package.json
pocketone pull pb_hooks pb_public/index.html
pocketone logs                         # the last 100 lines, then follow
```

## Deploy from CI

1. Sign in to the dashboard as a superuser and open **Settings → API tokens**.
2. Fill in a **Name**, such as `ci`.
3. Under **Scopes**, tick `deploy`. Add `remote` if CI will pass `--restart`.
4. Click **Create token** and copy it. It's shown only once.

In CI, set `P1_TOKEN` (or `P1_TOKEN_PROD` for the remote named `prod`) to that token, then run
`pocketone deploy --restart`. The commit is recorded from `$GITHUB_SHA`, `$CI_COMMIT_SHA` or git `HEAD`, or pass
`--commit`.

## From an agent

An agent with a terminal runs the same `pocketone deploy`. Over [MCP](https://pocketone.dev/docs/reference/mcp), it can restart the app
with `remote_restart` and read its log with `remote_logs`. There's no MCP tool that uploads files.

## What's sent

- **Sent:** every `pb_*` folder except `pb_data` (so `pb_hooks`, `pb_migrations`, `pb_pages`, `pb_public`,
  `pb_templates`), plus `package.json`, `bun.lock` or `bun.lockb`, and `patches/`. Dotfiles are included, such as
  `pb_public/.well-known`.
- **Packages:** with a `package.json`, the production `node_modules` tree goes too, so your hooks can `require` npm
  packages. Dev dependencies stay behind. `--no-node-modules` leaves the app's `node_modules` alone.
- **Never sent:** `pb_data` (your live data), `.git*` files and symlinks. Add your own patterns with `-e`.

Only superusers, or an API token with the `deploy` scope, can deploy. A token with `deploy:read` can only pull files
and read logs. Every deploy is recorded in the app's `_deploys` collection: who, when, what changed, the commit and your
note. Superusers can browse it in the dashboard.

## Change it

There's nothing to set in `pocketone.yaml`. Your project's remotes are kept in `.pocketbase/remotes.json`, which you
can commit. Your logins are saved in your home folder, never in the project. Every `pocketone.yaml` key is in the
[pocketone.yaml reference](https://pocketone.dev/docs/config-reference).

## How it works

- [Deploy](https://pocketone.dev/docs/extensions/deploy) gives the app its deploy API and adds the `deploy`, `dev`, `pull`, `logs` and
  `restart` commands.
- [Remotes](https://pocketone.dev/docs/extensions/remotes) keeps your remotes and logins, and gives the app its restart and logs.
- [Tokens](https://pocketone.dev/docs/extensions/tokens) makes the API tokens CI uses.
- The commands talk to the [deploy/v1](https://pocketone.dev/docs/contracts/deploy/v1) and [remote/v1](https://pocketone.dev/docs/contracts/remote/v1)
  contracts, so they work against any app or host that offers them.
