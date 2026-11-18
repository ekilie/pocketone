---
title: How the app runs
description: "What a PocketOne desktop app does on your users' machines: where their data lives, how upgrades migrate it, the window, and what each platform needs."
---

# How the app runs

A desktop app is your PocketOne app, with the standard extensions, its server on a private port, and a native window
in front.

## Each user's data

Every user has their own data, in their own app-data folder, named by your app's `id`:

| Platform | Data folder |
|---|---|
| macOS | `~/Library/Application Support/<id>/pb_data` |
| Windows | `%AppData%\<id>\pb_data` |
| Linux | `~/.config/<id>/pb_data` |

It's an ordinary `pb_data`, so everything that works on one, from backups to point-in-time restore, works here too.

## Upgrades

Your migrations run every time the app starts. Ship a new version with new migrations, and each user's data is
upgraded the first time they open it, the same way `pocketone serve` upgrades a server's.

## The server and the window

- The server listens on a free `127.0.0.1` port, chosen each time, so it never collides with anything else.
- The window opens once the server answers.
- Only one copy runs at a time.
- Your hooks, migrations and public files come from the executable, never from disk.
- There is no superuser, and the superuser installer page never opens: the people using a desktop app aren't its
  admins.

## What each platform needs

| Platform | Needs |
|---|---|
| macOS | macOS 11 or newer |
| Windows | The WebView2 runtime, which Windows 10 and 11 already have |
| Linux | The GTK 4 and WebKitGTK 6.0 libraries; without them the app says which package to install |

## Sharing your app

A build signed ad hoc runs on the Mac that built it. To give a macOS app to other people, sign and notarize it with
your own Apple Developer ID; unsigned, macOS asks them to approve it in System Settings first. Windows shows a
SmartScreen warning for unsigned apps until they're signed with a code-signing certificate. Signing from
`pocketone desktop build` is planned.
