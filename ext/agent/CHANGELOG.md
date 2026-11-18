# Agent changelog

Changes to `ext/agent`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- `$agent.stream(opts, onText)`: the answer a piece at a time as the model writes it, from the built-in model or your own API (asked to stream), and on a multisite host from its shared model; return `false` from `onText` to stop. It's agent.stream/v1, for code in the same process.
- New extension: a language model for every app, as agent/v1 and `$agent.complete()` in `pb_hooks` (with an optional JSON schema; the parsed answer comes back as `data`, and a model PocketOne runs can only write JSON that matches it). It answers from the app's own OpenAI-compatible API (Settings → Agent), else a model PocketOne runs itself (`pocketone.yaml`'s `agent.model`: `qwen3-0.6b`, `qwen3-1.7b`, `qwen3-4b`, an `hf:` file or a local `.gguf`; downloaded once, checked and cached), in pure Go with no cgo. On a multisite host whose `agent` section names a model, every Pocker instance shares it, with a per-instance limit on calls at once and a daily token allowance, unless the instance's admins set their own API.
