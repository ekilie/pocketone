# Agent Chat changelog

Changes to `ext/agent-chat`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New extension: stored conversations with the app's language model (agent.chat/v1). In `pb_hooks`: `$agent.newChat()`, `$agent.chat(id).send(text)` (and `.async`, and streaming with `onText`), `history()`, `title()`, `meta()`, `forget()`, `$agent.chats.list({meta})` and `$agent.chats.forget({meta})`. Chats aren't tied to users; the app's own `meta` finds them. Older messages fold into a summary, and the built-in model keeps each chat's state between turns.
