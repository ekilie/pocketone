---
owners: ["@benallfree"]
version: 0.0.0
title: "Multitenant"
group: saas
summary: "Planned. Accounts that own instances: quotas, owner logins, tokens and an API."
---

# tenants

Accounts that own instances, for hosting other people (level 4): each instance has an owner; accounts have quotas and
can be suspended; owners log in to their instances' dashboards (their login is kept in sync as a superuser); owners
get API tokens for the CLI and deploys; and an API your own tenant dashboard talks to (PocketHost's dashboard is one).

**Runs on:** [mothership](../mothership/README.md) (level 3) and [pocker](../pocker/README.md); requires the
[sandbox](../sandbox/README.md), since tenants run code you don't control.

> **Planned.** Nothing is published here yet. See [Create your own extension](../../docs/extensions.md).

## Install

Not available yet: Multitenant isn't in `pocketone` and has no Go module to add to a custom build.

## Configure

Nothing to configure yet. Its settings will be documented here when it ships.

## Use

Nothing to use yet. It is planned to give each instance an owner account, with quotas, suspension, dashboard sign-in for owners and API tokens for the CLI and deploys.

## Integrations

It is planned to work with these extensions:

- **Mothership:** adds an `owner` to instances and an accounts table through the mothership's schema hooks, and adds
  owner-derived values (superusers, blocked status, extension settings) to what edges receive through its feed hooks.
- **Settings for other extensions:** written into each instance's `extensions:` block, e.g. an owner's trusted IPs for the
  [firewall](../firewall/README.md) or a plan's retention for [pitr](../pitr/README.md). Those extensions don't know
  tenants exist.
- **Deploy:** supplies the tenant-token backend for the [deploy](../deploy/README.md) API.
- **Billing:** a billing provider hook; [tenant-billing-ls](../tenant-billing-ls/README.md) is the Lemon Squeezy one.

## Example

No example yet; it will come with the extension.

## License

MIT
