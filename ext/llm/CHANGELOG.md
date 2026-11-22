# LLM changelog

Changes to `ext/llm`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New standard extension, replacing Agent: cloud models for your app, one contract per kind (`llm.text/v1`, `llm.image/v1`, `llm.speech/v1`, `llm.transcribe/v1`, `llm.music/v1`, `llm.video/v1`, `llm.embed/v1`) and `$llm.<kind>` in page code. Providers are added in Settings → LLM with your own keys: Vultr, Cloudflare Workers AI, OpenRouter, or any OpenAI-compatible API. Every call names its model as `provider/model`. Music and video are jobs (`create`, `get`, `content`, or `wait`); `$llm.file()` turns a generated file into a PocketBase file.

### Upgrading

- Agent is gone, with its built-in model, `pocketone.yaml`'s `agent:` section and a multisite host's shared model. Replace `$agent.complete(opts)` with `$llm.text.complete({ model: "<provider>/<model>", ...opts })` after adding a provider in Settings → LLM; `$agent.stream` is `$llm.text.stream`. `thread`, `wait`, `warm`, `forget`, `threads` and `status` have no replacement (they were the built-in model's).
