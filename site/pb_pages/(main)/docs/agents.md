---
title: LLM skills for coding agents
description: Teach Claude Code, Cursor, Codex and other coding agents PocketOne and Pages with our Agent Skills and llms.txt.
---

# LLM skills for coding agents

PocketOne is new, so coding agents haven't seen much of it. Two things close that gap. Both are built from these docs, so they change when the docs change.

## Agent Skills

The `pocketone` plugin has two skills:

- **pocketone:** using PocketOne in an app: the `pocketone` binary and its commands, `pocketone.yaml`, deploying, every extension, and the app's pages in `pb_pages/` (routing, `ctx`, templates, layouts, auth).
- **pocketone-extensions:** writing your own extension and contributing it as a pull request: contracts, `extkit`, tests, and the README and changelog CI checks.

Each skill carries a copy of the docs in its `reference/` folder, so it works offline.

**Claude Code**

```
/plugin marketplace add pocketonehq/pocketone
/plugin install pocketone@pocketone
```

`/plugin marketplace update pocketone` picks up new versions.

**Other agents that read Agent Skills** (Cursor, Codex, Gemini CLI and others)

```bash
npx skills add pocketonehq/pocketone
```

Or copy a skill folder into your project yourself. The skills live in [`agent/skills/`](https://github.com/pocketonehq/pocketone/tree/main/agent/skills).

## llms.txt

For agents and tools that read the web:

- [pocketone.dev/llms.txt](/llms.txt) indexes every docs page with a one-line summary.
- [pocketone.dev/llms-full.txt](/llms-full.txt) holds every docs page in one Markdown file.

Pages' docs are part of this site, under [/docs/extensions/pages](/docs/extensions/pages), so both files include them.
