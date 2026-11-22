---
owners: ["@benallfree"]
version: 0.0.1
title: "LLM"
group: binary
summary: "Cloud models for your app: text, pictures, speech, transcription, music, video and embeddings from Vultr, Cloudflare, OpenRouter or any OpenAI-compatible API."
---

# LLM

LLM gives your app cloud models to call, one kind at a time: text, pictures, speech, transcription, music, video and
embeddings. You add providers in Settings → LLM, each with your own key, and every call names its model as
`provider/model`:

```ts
const r: LlmTextAnswer = await $llm.text.completeAsync({ model: "vultr/glm-5.2", prompt: "Summarize: " + body, reasoning: "off" })
const pic = await $llm.image.generateAsync({ model: "or/black-forest-labs/flux.2-klein-4b", prompt: "a lighthouse at dusk, watercolor" })
record.set("cover", $llm.file(pic.files[0]))
```

```go
r, err := textv1.Contract.Must(app).Complete(ctx, extkit.Internal, textv1.CompleteInput{
	Model: "vultr/glm-5.2", Prompt: "Summarize: " + body, Reasoning: "off"})
```

`vultr` is the name you gave a provider; what follows the first slash is that provider's own id for the model. Any
call can name any model of any provider, so one app can use two picture providers, or a cheap text model for labels and
a large one for writing.

Providers come in four types:

| Type | What it reaches |
|---|---|
| `vultr` | [Vultr Serverless Inference](https://docs.vultr.com/products/compute/serverless-inference): every kind, music and video included. |
| `cloudflare` | [Workers AI](https://developers.cloudflare.com/workers-ai/) on your Cloudflare account: text, pictures, speech, transcription and embeddings. |
| `openrouter` | [OpenRouter](https://openrouter.ai): hundreds of models behind one key; text, pictures, speech, transcription, video and embeddings. |
| `openai` | Any API shaped like OpenAI's: OpenAI itself, Groq, Together, or Ollama and llama-server on localhost. Add as many as you like. |

The answers come from your provider accounts, on your keys; PocketOne adds no limits or charges.

## Install

LLM is built into `pocketone` and on in every app and every instance of a host (a standard member). It does nothing
until you add a provider. In your own Go build:

```go
extkit.Setup(app, []extkit.Extension{llm.New()}, nil, nil)
```

The module is `pocketone.dev/ext/llm`.

## Configure

**Settings → LLM** (superusers) lists your providers. Press **New provider**, pick its type, give it a name (the part
before the slash in a model) and enter its key: an endpoint for `openai` providers (with its version path, like
`https://api.groq.com/openai/v1`), the account ID for Cloudflare (with a token that has Workers AI permission). Vultr,
Cloudflare and OpenRouter providers can't be saved without a key; an `openai` one can (a local Ollama has none). Each
provider's **Test** checks its key, typed or saved, without spending anything, and its check mark turns green when it
passes. Saving applies at once.

Keys stay on the server: the screen never shows a saved key again, and a key is never sent to an endpoint other than the
one it was entered for (change a provider's endpoint and you enter its key again). The settings are in the app's
`_params` table, row `llm`.

There's no pocketone.yaml configuration besides the switch: `llm: {enabled: false}` turns it off on a single app (on a
host, an instance's `extensions.llm`), and with it `$llm` and the `llm.*` contracts.

## Use

In page code it's `$llm.<kind>`. Every call that reaches a provider is async, and its name says so: await it in an async
handler (`await $llm.text.completeAsync({...})`). It runs off the app's JS VM, so the app keeps serving other requests
while a model works; there are no blocking versions to reach for. Each call names its `model`:

| Call | What it does |
|---|---|
| `$llm.text.completeAsync({ model, system?, prompt \| messages, schema?, maxTokens?, temperature?, stop?, reasoning?, options? })` | Answers: `{ text, data, reasoning, stopReason, model, usage }`. With `schema`, `data` is the parsed JSON. |
| `$llm.text.streamAsync(opts, onText, onReasoning?)` | The same, calling `onText` with each piece as it's written (`false` stops it). |
| `$llm.text.count(text \| messages)` | About how many tokens (four characters each). Local, so not async. |
| `$llm.image.generateAsync({ model, prompt, n?, size?, aspectRatio? })`, `edit({ ..., images })` | Pictures: `{ files, model, usage }`. |
| `$llm.speech.speakAsync({ model, text, voice?, format?, speed?, instructions? })`, `voices(model)` | Audio: `{ file, model, usage }`. |
| `$llm.transcribe.transcribeAsync({ model, audio, language?, prompt? })` | Text from audio: `{ text, language, model, usage }`. |
| `$llm.music.createAsync({ model, prompt, lyrics?, seconds?, wait? })`, `get(id)`, `content(id, index)` | Music, as a job (below). |
| `$llm.video.createAsync({ model, prompt, image?, seconds?, size?, aspectRatio?, wait? })`, `get(id)`, `content(id, index)` | Video, as a job. |
| `$llm.embed.embedAsync({ model, input, dimensions? })` | One vector per text: `{ vectors, model, usage }`. |
| `$llm.<kind>.models({ provider? })` | The models your providers offer of that kind, with prices where the provider lists them. |
| `$llm.file(file)`, `$llm.fileAsync(file)` | A generated file as a PocketBase file, for `record.set("field", ...)`; `fileAsync` also downloads one the provider gave only a URL for. |
| `$llm.chat.*` | Stored conversations, from the [LLM Chat](../llm-chat/README.md) extension. |

**`reasoning`** is how much a reasoning model thinks before answering: `"off"`, `"low"`, `"medium"`, `"high"`, or left
out for the model's own default. Many models think by default, and the thinking counts toward `maxTokens`: for short
jobs (labels, titles, JSON) say `"off"`, or the answer can come back empty with `stopReason: "length"`.

**`options`** passes a provider's own fields through unchanged (`seed`, `steps`, OpenRouter's `provider` routing...), for
what the common fields don't cover.

**Files** come back as `{ name, type, data, url }`: `data` is the bytes (base64 in JSON), `url` the provider's own copy
when it keeps one (it may expire). Audio to transcribe is sent the same way.

**Jobs:** music and video take a while. `create` starts one and returns `{ id, status }`; follow it with `get(id)` until
`status` is `done`, then fetch each of its `outputs` with `content(id, index)`. Or pass `wait: "5m"`: `create` then
resolves once the job is done, its `files` included. The id names the provider
(`vultr:music_02a2…`), so `get` finds it again after a restart; the provider keeps the job, not your app.

**Errors** have a code: `not_configured` (no providers yet), `unknown_model` (no model named, or no provider by that
name), `not_supported` (that provider type doesn't do this kind), `provider` (the provider's own error, its message
kept, with the provider's name in front), `timeout`.

**Over HTTP** each kind is a contract at `/api/_c/llm.<kind>/v1/` (`llm.text/v1/complete`, `llm.image/v1/generate`...),
for superusers and for API tokens with the `llm` scope (`llm:read` for `models`, `voices`, `get` and `content`); see
[llm.text/v1](../contracts/llm/text/v1/README.md) and the others in the [catalogue](../contracts/README.md).

What each provider type covers:

| | Text | Pictures | Speech | Transcription | Music | Video | Embeddings |
|---|---|---|---|---|---|---|---|
| `vultr` | yes | yes | yes, with voices | yes | yes (job) | yes (job) | yes |
| `cloudflare` | yes | yes | yes | yes | | | yes |
| `openrouter` | yes | yes (and edits) | yes, with voices | yes | | yes (job) | yes |
| `openai` | yes | yes | yes | yes | | | yes |

An `openai` provider's model list names only ids, so each model's kind is guessed from its name (`whisper` is
transcription, `tts` is speech, `embed` is embeddings, and so on; anything else is text).

## Integrations

| With | What happens |
|---|---|
| [LLM Chat](../llm-chat/README.md) | Stored conversations on top of `llm.text/v1`: `$llm.chat.new()`, `$llm.chat.open(id).sendAsync(text)`. |
| [Settings](../settings/README.md) | Settings → LLM is its `settings/v1` section. |
| [API Tokens](../api-tokens/README.md) | A token with the `llm` scope can call the contracts over HTTP, and MCP clients get them as tools. |
| [Multisite](../multisite/README.md) | Each instance has its own Settings → LLM, providers and keys. |

## Example

The example is a PocketBase app with LLM that, once you've added a provider named `vultr` in Settings → LLM, answers
one question at start-up and logs it.

```go
// Command example is a PocketBase app with the llm extension. Add a provider named "vultr" in Settings → LLM (or
// change the model below), run `go run . serve`, and watch the log for "llm: answer".
package main

import (
	"context"
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	textv1 "pocketone.dev/ext/contracts/llm/text/v1"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/llm"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app)
	if _, err := extkit.Setup(app, []extkit.Extension{llm.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		go func() {
			res, err := textv1.Contract.Must(app).Complete(context.Background(), extkit.Internal, textv1.CompleteInput{
				Model: "vultr/glm-5.2", Prompt: "In one sentence: what is PocketBase?", MaxTokens: 200, Reasoning: "off"})
			app.Logger().Info("llm: answer", "text", res.Text, "model", res.Model, "error", err)
		}()
		return e.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## License

MIT
