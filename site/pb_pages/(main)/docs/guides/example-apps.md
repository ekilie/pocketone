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
   the sources and the cost (`usage.costUSD`).
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
pb_migrations/1763800000_stories.js   the stories collection
pb_pages/
  index.ets            ask for a story; stories already told
  story/+api.ts        POST /story: saves the record, starts making the story
  story/[id].ets       progress while it's made, then the player
  _private/story.ts    the models, the limits and the six steps
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

A 1-minute story with three pictures takes about two minutes to make and costs about 8 cents on Vultr, most of it the
pictures and the music.
