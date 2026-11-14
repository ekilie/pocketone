---
title: Plugins
description: Every plugin in PocketOne, grouped by what it's for.
---

# Plugins

Each plugin is a Go module in [`ext/`](https://github.com/pockethost/pocketone/tree/main/ext) with its own README. Pages for each plugin are coming to
this site; until then the links go to GitHub.

## In every binary

These ship with `pocketone` and add no work to an app that doesn't use them.

| Plugin | What it does |
|---|---|
| [PITR](https://github.com/pockethost/pocketone/blob/main/ext/pitr/README.md) | Point-in-time restore with Litestream. On by default. |
| [Auto TLS](https://github.com/pockethost/pocketone/blob/main/ext/autotls/README.md) | Certificates on demand and wildcards through ACME. |
| [Bulk Mail](https://github.com/pockethost/pocketone/blob/main/ext/bulkmail/README.md) | Campaigns to your app's own users. |
| [Pages](https://github.com/pockethost/pocketone/blob/main/ext/pages/README.md) | Server-rendered pages from a `pb_pages` folder. |
| [Vacuum](https://github.com/pockethost/pocketone/blob/main/ext/vacuum/README.md) | Scheduled SQLite VACUUM. |
| [MCP](https://github.com/pockethost/pocketone/blob/main/ext/mcp/README.md) | An MCP endpoint at `/mcp` for agents. |
| [OAuth Server](https://github.com/pockethost/pocketone/blob/main/ext/oauth-server/README.md) | Your app as an OAuth provider. |
| [S3 Redirect](https://github.com/pockethost/pocketone/blob/main/ext/s3redirect/README.md) | File downloads as short-lived signed S3 redirects. |
| [Tokens](https://github.com/pockethost/pocketone/blob/main/ext/tokens/README.md) | Long-lived API tokens with scopes. |
| [Deploy](https://github.com/pockethost/pocketone/blob/main/ext/deploy/README.md) | `pocketone deploy`, `dev`, `pull`, `logs` and the deploy API. |
| [Remotes](https://github.com/pockethost/pocketone/blob/main/ext/remotes/README.md) | Named remotes and saved logins. |
| [Upgrade](https://github.com/pockethost/pocketone/blob/main/ext/upgrade/README.md) | `pocketone upgrade`. |
| [Config](https://github.com/pockethost/pocketone/blob/main/ext/config/README.md) | `pocketone.yaml` and `.env` secrets. |

## Run many instances

| Plugin | What it does |
|---|---|
| [Pocker](https://github.com/pockethost/pocketone/blob/main/ext/pocker/README.md) | Hundreds of PocketBase instances in one Go process, routed by hostname. |
| [Host](https://github.com/pockethost/pocketone/blob/main/ext/host/README.md) | Many instances behind one listener, run by one provider. |
| [Sandbox](https://github.com/pockethost/pocketone/blob/main/ext/sandbox/README.md) | Confines untrusted JS hooks, SQL and outbound network. |
| [Firewall](https://github.com/pockethost/pocketone/blob/main/ext/firewall/README.md) | Peer allow-lists, trusted proxies and rate limits. |
| [Edge](https://github.com/pockethost/pocketone/blob/main/ext/edge/README.md) | Forwards each request to the edge that owns the instance. |
| [Mothership](https://github.com/pockethost/pocketone/blob/main/ext/mothership/README.md) | A control plane and its edges, across regions. |
| [Schedules](https://github.com/pockethost/pocketone/blob/main/ext/schedules/README.md) | Scheduled webhooks that wake sleeping instances. |

## Services for your instances

| Plugin | What it does |
|---|---|
| [Mail Relay](https://github.com/pockethost/pocketone/blob/main/ext/mailrelay/README.md) | Outgoing mail for instances without SMTP. |
| [S3 Fallback](https://github.com/pockethost/pocketone/blob/main/ext/storage/README.md) | A default bucket for uploads and backups. |
| [Server Timing](https://github.com/pockethost/pocketone/blob/main/ext/servertiming/README.md) | `Server-Timing` headers for each step of a request. |

## Sell it as a service

| Plugin | What it does |
|---|---|
| [Multitenant](https://github.com/pockethost/pocketone/blob/main/ext/tenants/README.md) | Accounts that own instances, with quotas and their own logins. |
| [Multitenant Billing](https://github.com/pockethost/pocketone/blob/main/ext/tenant-billing/README.md) | Plans, quotas and a billing provider hook. |
| [Lemon Squeezy Adapter](https://github.com/pockethost/pocketone/blob/main/ext/tenant-billing-ls/README.md) | Lemon Squeezy as the billing provider. |

## For plugin authors

| Plugin | What it does |
|---|---|
| [Mail Trap](https://github.com/pockethost/pocketone/blob/main/ext/mailtrap/README.md) | A built-in inbox for dev mode. |
| [Dev Users](https://github.com/pockethost/pocketone/blob/main/ext/devusers/devusers.go) | Test users for local examples. |

Want to add one? Read [Writing a plugin](https://github.com/pockethost/pocketone/blob/main/docs/plugins.md).
