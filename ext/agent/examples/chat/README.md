# Docs Chat

A chat app about PocketOne, written as a PocketPages app with no Go: `pb_pages` and the stock `pocketone` binary. It
shows `$agent` from page code:

- **arming the model from disk:** [`_private/agent.js`](pb_pages/_private/agent.js) reads a preamble
  ([`preamble.md`](pb_pages/_private/preamble.md)) and context files ([`_private/context/`](pb_pages/_private/context),
  eleven pages of pocketone.dev) with `$os.readDir` / `$os.readFile`, splits them into sections, and for each question
  picks the three that match best (BM25, plus a few synonyms: people say "apps", the docs say "instances");
- **answering:** `$agent.complete({ system, messages })` with the preamble and those sections as the system prompt and
  the last few turns of the conversation;
- **structured output:** a second call, `$agent.complete({ schema: { type: 'array', ... } })`, suggests three follow-up
  questions; the model can only answer with an array of three strings;
- **status:** `$agent.status()` for the model's download progress.

```
pb_pages/
  index.ejs              the chat page
  chat/+api.js           POST /chat: the answer; GET /chat: status and what the app knows
  chat/suggest/+api.js   POST /chat/suggest: three follow-up questions (JSON schema)
  _private/agent.js      loads preamble + context, searches, calls $agent
  _private/preamble.md   who the assistant is and how it answers
  _private/context/*.md  what it knows
```

## Run it

```bash
cd ext/agent/examples/chat
pocketone superuser upsert admin@example.com password --dir=pb_data
pocketone serve --dir=pb_data
```

Open http://127.0.0.1:8090. The first start downloads the model named in [pocketone.yaml](pocketone.yaml)
(`qwen3-1.7b`, 1.8 GB) into your user cache folder; the page shows the progress. To use your own API instead, sign in at
`/_/` and set it in Settings → Agent.

To teach it something else, change `preamble.md` and put your own Markdown files in `_private/context/` (one
`Source: <url>` line at the top of a file links its sections). In dev mode (`--dev`) edits show up on the next question.

## What to expect

On an Apple M1 Max with `qwen3-1.7b`, answers take 4 to 12 seconds, and the follow-up questions a second or two more.
A small x86 server is several times slower; `qwen3-0.6b` is about three times faster than 1.7B and less careful. The
answers are only as good as the three sections it finds, so a question the context doesn't cover gets "I don't know"
or a near miss. There's no streaming yet (agent/v1 answers in one piece), so the page shows a timer while it waits.
