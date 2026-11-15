# schedules

Scheduled webhooks per instance, called in-process even while the instance sleeps, with a catch-up run after downtime.

**Runs on:** [pocker](../multisite-provider-pocker/README.md) (multitenant).

> **Planned.** Nothing is published here yet. This folder will hold the plugin's Go module
> (`github.com/pockethost/pocketone/ext/multisite-schedules`), its tests and a minimal `example/main.go` that builds and shows the
> feature. See [Create your own plugin](../../docs/plugins.md).

## Install

Not available yet: Schedules isn't in `pocketone` and has no Go module to add to a custom build.

## Configure

Nothing to configure yet. Its settings will be documented here when it ships.

## Use

Nothing to use yet. It is planned to let each instance schedule webhooks that fire on time even while the instance is asleep, and to run missed ones once after downtime.

## Integrations

It is planned to run on [pocker](../multisite-provider-pocker/README.md), which hosts many instances in one process and calls each schedule without waking the instance first.

## Example

No example yet; it will come with the plugin.

## License

MIT
