Source: https://pocketone.dev/docs/features

# New in PocketOne

PocketOne does everything PocketBase does, and adds the features below. They're built in and on by default, whether
you run the `pocketone` binary yourself or a host runs your app. There's nothing to install.

| | PocketBase gives you | PocketOne adds |
|---|---|---|
| [Point-in-time restore](https://pocketone.dev/docs/features/restore) | Backups you schedule, restored whole | Every change recorded; restore to any second in the last 72 hours, and undo the restore |
| [Deploy and live sync](https://pocketone.dev/docs/features/deploy) | Copy files to the server yourself, then restart | `pocketone deploy` uploads only what changed; `pocketone dev` syncs a folder both ways and streams logs |
| [Remote CLI](https://pocketone.dev/docs/features/remote) | A CLI for the machine that holds `pb_data` | Admin commands against any app, from your terminal, a script or an agent |
| [API tokens and OAuth](https://pocketone.dev/docs/features/tokens) | Auth tokens with the account's full rights | Named, scoped tokens you revoke one by one; your app is an OAuth server |
| [MCP for AI agents](https://pocketone.dev/docs/features/mcp) | No MCP | `/mcp` on every app, with OAuth sign-in and your own tools from `pb_hooks` |
| [Bulk mail](https://pocketone.dev/docs/features/bulk-mail) | Account mail: verification, password reset | Campaigns from the dashboard, with unsubscribes, suppression and pacing |
| [PocketPages sites](https://pocketone.dev/docs/features/sites) | Static files from `pb_public` | Server-rendered pages from `pb_pages`, with direct access to your records |
| [QuickJS hooks](https://pocketone.dev/docs/quickjs) | Hooks on goja | The same JS API with imports, TypeScript, async handlers, shared globals and hard limits |
| [pocketone.yaml](https://pocketone.dev/docs/pocketone-yaml) | Flags on every start | One config file, with every extension's options and a `.env` for secrets |

Each page shows the feature in the dashboard, from your terminal and from an agent. For how a feature works or how to
build on it, follow its **How it works** links to the [extension](https://pocketone.dev/docs/extensions) pages.

## Also built in

Smaller additions you'll notice without a page of their own:

- **Mail you can see in dev mode:** with `serve --dev` and no SMTP settings, mail is caught and shown at
  `http://127.0.0.1:8025` instead of being sent ([Mail Trap](https://pocketone.dev/docs/extensions/mailtrap)).
- **S3 redirects:** files kept in S3 download straight from the bucket through short-lived signed links
  ([S3 Redirect](https://pocketone.dev/docs/extensions/s3redirect)).
- **Restylable sign-in pages:** copy the OAuth sign-in screens into your app with `pocketone template eject` and edit
  them ([Templates](https://pocketone.dev/docs/extensions/templates)).
- **Settings and cache:** one settings screen and one shared memory budget for every feature
  ([Settings](https://pocketone.dev/docs/extensions/settings), [Cache](https://pocketone.dev/docs/extensions/cache)).

Coming from PocketBase? [What stays the same](https://pocketone.dev/docs/pocketbase).
