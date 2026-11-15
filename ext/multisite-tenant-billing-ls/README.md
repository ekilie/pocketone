# tenant-billing-ls

The [Lemon Squeezy](https://www.lemonsqueezy.com) adapter for [tenant-billing](../multisite-tenant-billing/README.md): checkout
links, signature-verified webhooks mapped to subscriptions, renewals, cancellations and refunds, and the Lemon Squeezy
side of the reconcile job. Plans, quotas and applying them to accounts live in tenant-billing.

**Runs on:** [tenant-billing](../multisite-tenant-billing/README.md) (level 4). It implements tenant-billing's provider hook, so
other providers can be adapters of their own.

> **Build it.** Nothing is published here yet. PocketHost's own Lemon Squeezy billing (in its legacy Node mothership)
> moves here. See [Create your own plugin](../../docs/plugins.md).

## Install

Not available yet: Lemon Squeezy Adapter isn't in `pocketone` and has no Go module to add to a custom build.

## Configure

Nothing to configure yet. Its Lemon Squeezy settings (store, API key, webhook secret) will be documented here when it ships.

## Use

Nothing to use yet. It is planned to create checkout links and turn Lemon Squeezy webhooks into subscription changes.

## Integrations

It is planned to implement the provider hook of [tenant-billing](../multisite-tenant-billing/README.md), which owns plans, quotas and applying them to accounts. Other payment providers can be adapters of their own.

## Example

No example yet; it will come with the plugin.

## License

MIT
