Source: https://pocketone.dev/docs/overview

# What is PocketOne?

PocketOne is a community fork of [PocketBase](https://pocketbase.io). It tracks upstream PocketBase and keeps its own
changes to the core small: only the hooks that let extensions extend and override PocketBase. Every feature lives in an
extension.

It's maintained by [Ben Allfree](https://github.com/benallfree) and contributors. Ben also created
[PocketHost](https://pockethost.io), which runs PocketOne in production, but PocketOne is its own project, open to
anyone who wants to help. PocketOne is not affiliated with or endorsed by the PocketBase project.

## Why a fork

PocketBase is a remarkable piece of software, and a lot of that comes from how it's made: one author, a deliberately
narrow scope, and a careful, steady pace. That focus is a big part of why it's so good, and how PocketBase is run is
Gani's call to make.

It also means PocketBase isn't set up to take larger contributions from the community, so ideas people want to build
have nowhere to land upstream: hooks that can do more, apps that run many instances, features anyone can write and
share. PocketOne is a different path for that work. It stays close to upstream and passes PocketBase's own tests, keeps
its changes to PocketBase small, and puts everything else in extensions that anyone can build on, improve and send
back.

## What it adds

Run `pocketone` the way you run `pocketbase`. These are the differences you'll notice; [New in PocketOne](https://pocketone.dev/docs/features) compares each one with PocketBase.

### Point-in-time restore, on by default

Your database is replicated with Litestream, so you can restore it to any moment from **Settings → Backups** or with
`pocketone pitr restore`. Turn it off with `--pitr=false`.

### Mail you can see in dev mode

In dev mode (`serve --dev`), an app with no SMTP settings doesn't send through the system's `sendmail`. The mail is
caught and shown at `http://127.0.0.1:8025`.

### Server-rendered pages

Put `.ejs` and Markdown files in a `pb_pages` folder and they're served as pages, with no build step. This site is
one. `pocketone pages create myapp` starts a new site from a starter kit. Full docs are under
[Pages](https://pocketone.dev/docs/extensions/pages/overview).

### A config file

Settings can live in `pocketone.yaml`, with secrets in a `.env` file beside it that your JS hooks never see. Without
the file, PocketOne behaves like stock PocketBase. `pocketone config` lists every section, and
`pocketone config check` checks a file. The [config reference](https://pocketone.dev/docs/config-reference) lists every section.

### Commands for working with remote apps

- `pocketone login`, `remote` and `link`: save logins to your apps.
- `pocketone deploy`, `dev`, `pull` and `logs`: ship `pb_hooks`, `pb_migrations` and `pb_public` to an app and
  watch it.
- `pocketone upgrade`: move to the latest release, keeping the old executable.
- `pocketone remote …`: every operation of every contract an app offers is a command, such as
  `pocketone remote restore to --at -2h`. The same operations are REST endpoints and MCP tools.
  [Remote CLI](https://pocketone.dev/docs/features/remote) explains it, and the [CLI reference](https://pocketone.dev/docs/reference/cli) lists every command.

### Agents and API tokens

- An MCP endpoint at `/mcp` lets agents such as Claude manage your app and call its tools.
- Long-lived API tokens (`pbt_…`) with scopes, for scripts and integrations.
- Your app can act as an OAuth provider; `pocketone login` and MCP sign-in use it.

### Extensions that fit together

PocketOne's extensions meet through contracts: named, versioned promises like `restore/v1` that any extension can
keep. The dashboard, the `pocketone` command, agents and your `pb_hooks` call the contract, so you can swap the
extension behind it without changing anything else. [How PocketOne fits together](https://pocketone.dev/docs/how-pocketone-fits-together) explains it.

### Everything else

Automatic TLS certificates, bulk mail to your own users, scheduled VACUUM, signed S3 download redirects, and more. See
[all extensions](https://pocketone.dev/docs/extensions).

## What stays the same

The admin dashboard, collections, rules, the REST and realtime APIs, JS hooks and migrations all work as in PocketBase.
For those, the [PocketBase docs](https://pocketone.dev/docs/pocketbase) are the reference.

## How it's built

- **A small core.** PocketBase's own code changes only to add hooks. Each change is marked in the source, reviewed by
  someone who didn't write it, and kept minimal so upstream releases merge cleanly. You can review the complete core
  changes [here](https://github.com/pocketonehq/pocketone#changes-we-made-to-pocketbase).
- **Everything else is an extension.** Extensions live in `ext/` in the repo. Each is its own Go module, with a README and a
  small example.
- **Off until you use it.** The extensions in the `pocketone` binary add no work to an app that doesn't use them.
- **One app or hundreds.** An extension written against the extension contract works the same on a single app and on every
  instance under a multi-instance host.

## Open to contributions

PocketOne is open to everyone. Ask questions and share ideas in
[GitHub Discussions](https://github.com/pocketonehq/pocketone/discussions), report bugs in
[Issues](https://github.com/pocketonehq/pocketone/issues), and send extensions as pull requests. Start with
[Writing an extension](https://pocketone.dev/docs/writing-an-extension).

## Where to next

- [Install PocketOne](https://pocketone.dev/docs/installation)
- [Browse the extensions](https://pocketone.dev/docs/extensions)
