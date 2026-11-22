# LLM changelog

Changes to `ext/llm`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Upgrading

- Guest calls (`$llm.for(e)` for a visitor who isn't signed in) are refused until Settings → LLM has a limit for all guests together. An app that answers visitors who aren't signed in adds one there (say $1 a day) after upgrading. Plain `$llm` calls aren't guest calls and aren't affected.

### Added

- Limits in Settings → LLM, checked before every call: the whole app, each signed-in user, each guest, or all guests together, at most so many US dollars, output tokens, input tokens or calls a UTC day or month, optionally for one kind or one model. A used-up limit refuses the call with code `limit` (429), naming the limit and when it resets ([Limits](docs/guides/limits.md)).
- `$llm.for(e)`: the same API as `$llm`, with every call counted for the request's signed-in user, or else its guest (a signed cookie, by IP until the browser sends it back), so per-user and per-guest limits apply. Pass a page's `ctx` or a route's `e`.
- Every call is recorded in the `_llm_calls` collection (kind, model, whom it was for, tokens, seconds, cost, and which limit refused it); Settings → LLM shows today's and this month's spend.

### Changed

- Settings → LLM is kept in the app's key-value store (kv/v1) under the private name `_llm/settings`; the old
  `_params` row `llm` moves there by itself the first time the settings are read. Nothing to do.

## 0.0.1 (2026-11-22)

### Added

- New standard extension, replacing Agent: cloud models for your app, one contract per kind (`llm.text/v1`, `llm.image/v1`, `llm.speech/v1`, `llm.transcribe/v1`, `llm.align/v1`, `llm.music/v1`, `llm.video/v1`, `llm.embed/v1`) and `$llm.<kind>` in page code, where every call that reaches a provider is async and named so (`await $llm.text.completeAsync({...})`, `generateAsync`, `speakAsync`...): it runs off the app's JS VM, so the app keeps serving other requests while a model works. Providers are added in Settings → LLM with your own keys: Vultr, Cloudflare Workers AI, OpenRouter, or any OpenAI-compatible API. Every call names its model as `provider/model`. Music and video are jobs (`createAsync`, `getAsync`, `contentAsync`, or `wait`); `$llm.file()` turns a generated file into a PocketBase file. Every result's `usage.costMicroUSD` is what the call cost, in millionths of a US dollar (a whole number, `0` when free or not known): the provider's figure when it reports one, else estimated from the model's listed prices (`usage.costEstimated`).

### Upgrading

- Agent is gone, with its built-in model, `pocketone.yaml`'s `agent:` section and a multisite host's shared model. Replace `$agent.complete(opts)` with `await $llm.text.completeAsync({ model: "<provider>/<model>", ...opts })` (in an async handler) after adding a provider in Settings → LLM; `$agent.stream` is `await $llm.text.streamAsync`. There are no blocking versions. `thread`, `wait`, `warm`, `forget`, `threads` and `status` have no replacement (they were the built-in model's).

