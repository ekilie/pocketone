---
title: Pictures, speech and music
description: "Generate pictures, narration, music and video with $llm and save them on your records as files."
---

# Pictures, speech and music

Pictures, audio and video come back as files: `{ name, type, data }`, with the bytes in `data`. `$llm.file()` turns one
into a PocketBase file, so it goes straight into a record's file field.

## Pictures

```ts
const r = await $llm.image.generateAsync({ model: "vultr/z-image-turbo", prompt: "a small lighthouse at dusk, watercolor" })
const page = new Record($app.findCollectionByNameOrId("pages"))
page.set("picture", $llm.file(r.files[0]))
$app.save(page)
```

```go
r, err := imagev1.Contract.Must(app).Generate(ctx, extkit.Internal, imagev1.GenerateInput{
	Model: "vultr/z-image-turbo", Prompt: "a small lighthouse at dusk, watercolor"})
if err != nil {
	return err
}
f, _ := filesystem.NewFileFromBytes(r.Files[0].Data, r.Files[0].Name)
page.Set("picture", f)
```

`n` asks for several, `size` (`"1024x768"`) or `aspectRatio` (`"16:9"`) shapes them, as the model allows.
`$llm.image.editAsync({ model, prompt, images: [file] })` changes pictures you give it (on OpenRouter).

## Speech

```ts
const r = await $llm.speech.speakAsync({ model: "vultr/qwen3-tts-12hz-1.7b", text: storyPage, voice: "vivian", instructions: "slow and calm" })
page.set("narration", $llm.file(r.file))
```

`$llm.speech.voicesAsync(model)` lists a model's voices, where the provider says. `format` picks `mp3` (the default), `wav`,
`opus` or what else the model makes.

## Transcription

Send audio as a file with its bytes in `data` (base64) and a name that says its format:

```ts
const r = await $llm.transcribe.transcribeAsync({ model: "vultr/whisper-large-v3-turbo", audio: { name: "memo.mp3", data: base64Audio } })
// r.text: what was said
```

## Music and video

These take a while, so they're jobs. The simplest way is to wait for it:

```ts
const job = await $llm.music.createAsync({ model: "vultr/ace-step-1.5", prompt: "gentle lullaby, music box and soft strings", seconds: 60, wait: "5m" })
if (job.status === "done") page.set("music", $llm.file(job.files[0]))
```

Or start it and look again later, when you'd rather not hold a request open:

```ts
const job = await $llm.music.createAsync({ model: "vultr/ace-step-1.5", prompt: "gentle lullaby", seconds: 60 })
page.set("music_job", job.id) // "vultr:music_02a2…"

// later, in a cron or the next request
const now = await $llm.music.getAsync(page.getString("music_job"))
if (now.status === "done") page.set("music", $llm.file(await $llm.music.contentAsync(now.id, 0)))
```

Video works the same with `$llm.video` (on Vultr and OpenRouter); `image` gives it a first frame to start from.

## What it costs

Media is billed per picture, per second or per character by your provider. `usage.costUSD` says what a call cost when
the provider reports it, and `$llm.image.modelsAsync()` and the others list each model's prices.
