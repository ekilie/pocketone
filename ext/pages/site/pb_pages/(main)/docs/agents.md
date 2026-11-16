---
title: Coding agents
description: Teach Claude Code, Cursor, Codex and other coding agents PocketPages v1 with the pocketpages Agent Skill and llms.txt.
---

# Coding agents

Coding agents tend to write PocketPages as if it were a Node framework, or as if it were the 0.x npm package. The `pocketpages` Agent Skill teaches them v1: `pb_pages/`, `ctx`, routing, templates, layouts, auth and deploying. It's built from these docs and the v1 spec, so it changes when they change.

**Claude Code**

```
/plugin marketplace add pockethost/pocketone
/plugin install pocketone@pocketone
```

The plugin also has the `pocketone` skill, for the binary, `pocketone.yaml` and its extensions.

**Other agents that read Agent Skills** (Cursor, Codex, Gemini CLI and others)

```bash
npx skills add pockethost/pocketone
```

Or copy [`agent/skills/pocketpages`](https://github.com/pockethost/pocketone/tree/main/agent/skills/pocketpages) into your project.

## llms.txt

- [pocketpages.dev/llms.txt](/llms.txt) indexes every docs page with a one-line summary.
- [pocketpages.dev/llms-full.txt](/llms-full.txt) holds every docs page in one Markdown file.
