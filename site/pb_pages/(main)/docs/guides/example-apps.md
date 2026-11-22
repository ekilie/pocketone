---
title: Example apps
description: Two PocketOne apps you can try live and read end to end - Docs Chat and Bedtime Story - what they do, where to find them, and how each is built.
---

# Example apps

Two small apps show what a PocketOne app looks like when it uses cloud models. Each is a PocketPages app with no Go:
a `pb_pages` folder (and, for one, a migration) run by the stock `pocketone` binary. Both run live on PocketHost, and
their source is in the PocketOne repository, ready to copy.

| App | Try it | Source |
|---|---|---|
| **Docs Chat**: ask about PocketOne and PocketBase, answered from both sets of docs | [learn.pocketone.dev](https://learn.pocketone.dev) | [`ext/llm/examples/chat`](https://github.com/pocketonehq/pocketone/tree/main/ext/llm/examples/chat) |
| **Bedtime Story**: one topic told slowly and in depth, read aloud with pictures and music | [sleep.pocketone.dev](https://sleep.pocketone.dev) | [`ext/llm/examples/bedtime`](https://github.com/pocketonehq/pocketone/tree/main/ext/llm/examples/bedtime) |

Both use [`$llm`](/docs/extensions/llm), PocketOne's cloud models, on providers added in Settings → LLM.

## Run one yourself

```bash
cd ext/llm/examples/bedtime        # or ext/llm/examples/chat
pocketone superuser upsert admin@example.com password --dir=pb_data
pocketone serve --dir=pb_data
```

Open the dashboard at `http://127.0.0.1:8090/_/`, add a provider in Settings → LLM (both apps default to Vultr models
under a provider named `vultr`), then open `http://127.0.0.1:8090`. To put your copy online, `pocketone link` your own
app and `pocketone deploy` ([Deploy and live sync](/docs/features/deploy)).

## Docs Chat

A chat page that answers questions about PocketOne and PocketBase, citing the docs it used and showing what each answer
cost.

```
pb_pages/
  index.ejs              the chat page
  chat/+api.ts           POST /chat: the answer as server-sent events; GET /chat: what the app knows
  chat/suggest/+api.ts   POST /chat/suggest: three follow-up questions
  _private/llm.ts        the model's name, the docs, search, the call to the chat
  _private/preamble.md   who the assistant is and how it answers
```

**What it knows.** `_private/llm.ts` loads two sets of docs in the background and refreshes them every ten minutes:
PocketOne's user docs from this site's `llms.txt` and `llms-full.txt`, and PocketBase's own docs from the Markdown copy
PocketOne keeps of pocketbase.io. It splits them into sections and indexes the words in each.

**Answering a question.**

1. `POST /chat` finds the three sections that match the question best (a BM25 search, with a few synonyms), each
   labelled with the docs it came from.
2. It sends the preamble, the conversation so far, and the question with those excerpts to the model, through a stored
   chat: `$llm.chat.open(id).sendAsync(question, { context, onText })`. The chat keeps the conversation, so the page
   holds only the chat's id.
3. Each piece of the answer is forwarded to the browser as it's written, with `ctx.res.sse()`, then a final event with
   the sources and the cost (`usage.costMicroUSD`, in millionths of a dollar).
4. A second call, `$llm.text.completeAsync({ schema })`, suggests three follow-up questions as JSON.

Every `$llm` call is awaited in an async handler, so the model works off the app's JS engine and the app keeps serving
other requests while an answer streams. Only the excerpts that match are sent, so each question is small and cheap, and
because the start of each prompt repeats, providers bill it at their cheaper cached rate.

## Bedtime Story

You give it a topic, or leave it empty and it picks one: the universe, the mind, a moment in history. A minute or two
later it plays the story: one narrator reading, pictures that slowly pan and zoom, the words lit up as they're said,
and quiet music underneath. You choose the length and how many pictures. Each story is saved, so it can be replayed and
shared.

```
pb_migrations/         stories, their listens, each call's cost, the saved tuning, audio samples
pb_hooks/
  admin_plugins.pb.js  registers the tuning screen with the dashboard
pb_admin_ext/
  bedtime/main.js      the tuning screen: Settings → Bedtime Story
pb_pages/
  index.ets            ask for a story, or Inspire me; stories told, most listened to first
  inspire/+api.ts      POST /inspire: a topic no story has covered yet
  view/+api.ts         POST /view: counts a listen
  story/+api.ts        POST /story: saves the record, starts making the story
  story/[id].ets       progress while it's made, then the player
  admin/tuning/+api.ts GET/POST /admin/tuning: the tuning (superusers only)
  admin/sample/+api.ts POST /admin/sample: tries a piece with unsaved tuning (superusers only)
  _private/story.ts    the default tuning, the limits and the six steps
```

**Making a story takes longer than a request should wait**, so `POST /story` saves a `stories` record with status
`queued`, starts the work without awaiting it, and answers at once with the record's id. The work runs on in the
background, saving its progress on the record after each step. The story page polls the record through PocketBase's
records API (the collection's list and view rules are open, so anyone with the link can watch and play), and reloads
into the player when the status is `done`.

**The six steps** (`_private/story.ts`):

1. **Write.** `$llm.text.completeAsync` with a JSON schema returns the title, the topic, a description of what recurs
   in the pictures, and the story one sentence per item, in the prompt's language.
2. **Read it aloud.** `$llm.speech.speakAsync` records one voiceover of the whole story, so the narrator's voice stays
   the same throughout.
3. **Align.** `$llm.align.alignAsync` takes the voiceover and the sentences and returns when each sentence and each word
   is said. Nothing is transcribed: every word that comes back is one that was written.
4. **Place the pictures.** The timed transcript goes back to the text model, which chooses the sentence where each
   picture begins and what it shows, plus one style line for all of them and a prompt for the music.
5. **Paint** the pictures with `$llm.image.generateAsync`, all at once, and
6. **compose** the music with `$llm.music.createAsync({ wait })`, at the same time.

The files (the voiceover, the pictures, the music) are saved on the record with `$llm.file()`, and the story with its
timings in a JSON field. The pictures look alike because every prompt repeats the same style line and the same
description of what recurs.

**The player** (`story/[id].ets`) is plain HTML, CSS and a little JavaScript. One audio element plays the voiceover and
drives everything else from its current time: the picture whose start has passed fades in, its slow pan and zoom is a
CSS animation, placed by a negative `animation-delay` after a seek, and the caption shows the sentence being read with
each word brightening at its start time. The music loops under the voice at low volume. A scrubber, the space bar and
the arrow keys move through the story.

**Inspire me** sends the titles and topics of every story told so far, and the topics it already suggested on this
page, to the text model, which suggests one that none of them covers. **Listens:** after 10 seconds of actual listening
(time playing, not scrubbing), the player posts `/view`, which adds one to the story's `views` in a single SQL update so
listens at the same moment all count. The front page lists stories by it.

### Tuning it from the dashboard

Everything that shapes a story (the narrator's voice, what the models are told, how pictures and music are asked for,
which models run and how much a day may cost) can be changed without touching code, from **Settings → Bedtime Story**
in the PocketBase dashboard (`/_/`, superusers only). Each part has a **Sample** button, so you hear or see a change
before any listener does.

| Part | What you set | Sample |
|---|---|---|
| **Narrator** | **Voice:** `default`, a calm middle-aged woman with a low, clear voice, generated afresh each call; or `design`, a new voice made from the description in Instructions. **Instructions:** how `default` reads, or the voice itself for `design` ("A deep, calm older man's voice, slow and soft"). **Seed:** the same seed gives the same voice every time; empty lets it vary a little. **Speed:** 1 is normal. **Sample text:** what Sample reads. | Reads the sample text in that voice. `design` with no description is refused. |
| **Writing the story** | The writer's instructions. Placeholders: `{minutes}`, `{minWords}`, `{maxWords}`, `{anyTopic}`. Keep the field names it mentions (title, language, topic, look, sentences): the shape of the answer is fixed. | Writes a 1-minute story on the topic you give (or one it picks) and shows its sentences. |
| **Choosing the pictures** | The instructions that turn the timed transcript into pictures, their style and the music prompt (placeholder `{n}`), and the words added after every picture's prompt. | Paints one picture in the style you give. |
| **Music** | The words added after every story's music prompt, and how many seconds of music to make (it loops). | Plays 20 seconds for the music prompt you give. |
| **Inspire me** | The instructions for suggesting a topic no story has covered. | Suggests one. |
| **Models and budget** | Each step's model as `provider/model` (a provider in Settings → LLM), and the daily budget in US dollars, with what's been spent today. | |

**Sample** uses what's on the screen, saved or not, and shows which model answered, how long it took and what it cost.
Samples are recorded with every other model call, but the daily budget never stops them. **Save changes** stores the
tuning; every story started after that uses it. **Put back the defaults** forgets it.

**What stories remember.** Since the tuning can change, each story keeps how it was made: the seconds it took and the
models and voice that made it. The story's splash shows them with its date and cost, for example "Made Nov 22, 2026 in
21 s for $0.057", then writing `vultr/mimo-v2.6-flash`, voice `vultr/auk`, narrator `default, seed 7`.

**How it's built.**

- **Where the tuning lives.** `_private/story.ts` holds the defaults (`DEFAULT_TUNING`, with models overridable from
  the environment). A save writes the changed values to a `settings` record (key `tuning`, open to no one but
  superusers and the app), and `tuning(app)` merges it over the defaults each time a story or a suggestion starts.
  `/admin/sample` merges the screen's unsaved values over that again, so a sample never changes what listeners get.
- **The screen is a dashboard extension**, registered from `pb_hooks` the way
  [PocketHost's guide](https://pockethost.io/docs/admin-extensions) shows. The hook hands PocketBase the folder
  holding the screen:

  ```js
  // pb_hooks/admin_plugins.pb.js
  $app.onServe().bindFunc((e) => {
    e.uiExtensions.push({ name: "bedtime", fs: $os.dirFS(`${__hooks}/../pb_admin_ext/bedtime`) })
    e.next()
  })
  ```

  PocketBase serves the folder at `/_/extensions/bedtime/` and loads its `main.js` into the dashboard. `main.js` adds
  the Settings entry (`app.store.settingsNavGroups`) and the page (`app.routes.superuserOnly("#/settings/bedtime", ...)`),
  built from the dashboard's own pieces, and calls `/admin/tuning` and `/admin/sample` with `app.pb.send`, which carries
  the signed-in superuser's token. Those two endpoints answer only superusers.
- **Audio samples are saved, then played.** The dashboard plays media only from its own address (its
  Content-Security-Policy blocks `data:` audio), so each audio sample is saved in a `samples` collection, viewable by
  its random id only and trimmed to the last 20, and played from `/api/files/...`. Picture samples come back inline.

### Guards for a public copy

Every model call's cost (`usage.costMicroUSD`) is saved as a row; once the day's total
(from 00:00 UTC) reaches the budget, $1 by default, the form turns off with "Out of tokens for today" and the API
refuses new stories. Before anything is made, the app checks that each model's provider is set up, answering and
offering that model, and refuses the same way when it isn't. Stories are at most 5 minutes with 5 pictures, held both
in the form and in the API.

A 1-minute story with three pictures takes under a minute to make and costs a few cents on Vultr, most of it the
pictures and the music.
