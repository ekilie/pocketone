---
title: Features
description: What every PocketOne app can do on top of PocketBase, as step-by-step walkthroughs.
---

# Features

Every PocketOne app can do everything PocketBase does, plus the features below. They're built in and on by default,
whether you run the `pocketone` binary yourself or a host runs your app. You don't need to install anything.

Each page walks through one task in the dashboard, from your terminal and from an agent. To find out how a
feature works or how to build on it, follow the **How it works** links to its [extension](/docs/extensions) pages.

## Protect your data

- [Undo a mistake](/docs/features/restore): put your data back as it was at any moment, from the dashboard.

## Also built in

These work behind the scenes, so they have no walkthrough:

- **Settings:** one place for the settings superusers change for each feature ([Settings](/docs/extensions/settings)).
- **Page templates:** the sign-in screens features show can be copied into your app with `pocketone template eject` and restyled ([Templates](/docs/extensions/templates)).
- **Cache:** one shared memory budget for the caches features keep ([Cache](/docs/extensions/cache)).
- **S3 redirects:** files kept in S3 download straight from the bucket through short-lived signed links ([S3 Redirect](/docs/extensions/s3redirect)).
