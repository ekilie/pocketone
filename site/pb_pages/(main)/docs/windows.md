---
title: Install on Windows
description: Install PocketOne on Windows without Scoop, from a release zip, with no administrator rights.
---

# Install on Windows

The quickest way is [Scoop](/docs/installation#scoop-windows). Without it, install from a release zip.

Releases include 64-bit Windows builds: the zip ending in `_windows_amd64.zip` for most PCs, `_windows_arm64.zip` for
ARM ones. The executable inside is `pocketone.exe`. Download one from
[Releases](https://github.com/pocketonehq/pocketone/releases), then in PowerShell, from the folder you downloaded it to:

```powershell
$dir = "$env:LOCALAPPDATA\Programs\pocketone"
Expand-Archive pocketone_*_windows_amd64.zip -DestinationPath $dir -Force
Unblock-File "$dir\pocketone.exe"
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$dir", "User")
```

This puts PocketOne in a folder of your own, so it needs no administrator rights, and adds that folder to your user
`PATH`. Open a new terminal, then check it:

```powershell
pocketone --version
```

`Unblock-File` clears the "downloaded from the internet" mark. The executable isn't code-signed, so if you start it
from File Explorer instead, SmartScreen may stop it: choose **More info**, then **Run anyway**.

## Upgrade

```powershell
pocketone upgrade
```

It keeps the old executable as `pocketone-<version>.exe` in the same folder.

The commands in the rest of the docs are the same in PowerShell and in Command Prompt. Continue with
[Run it](/docs/installation#run-it).
