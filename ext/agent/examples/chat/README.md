# Docs Chat

A chat app about PocketOne, written as a PocketPages app with no Go: `pb_pages` and the stock `pocketone` binary. It
shows `$agent` from page code:

- **arming the model from disk:** [`_private/agent.ts`](pb_pages/_private/agent.ts) reads a preamble
  ([`preamble.md`](pb_pages/_private/preamble.md)) and context files ([`_private/context/`](pb_pages/_private/context),
  eleven pages of pocketone.dev) with `$os.readDir` / `$os.readFile`, splits them into sections, and for each question
  picks the three that match best (BM25, plus a few synonyms: people say "apps", the docs say "instances");
- **a stored chat, answered as it's written:** `$agent.newChat()` once, then
  `$agent.chat(id, { system }).send(question, { context, onText })` each turn (the agent-chat extension): the chat
  keeps the conversation (the page holds only its id), sends the preamble, the turns so far and the question with its
  excerpts after it, and keeps the question as sent, so each turn's prompt starts like the last one and the model
  reads only the new question; [`chat/+api.ts`](pb_pages/chat/+api.ts) forwards each piece to the browser with
  `ctx.res.sse()`;
- **structured output:** a second call, `$agent.complete({ schema: { type: 'array', ... } })`, suggests three follow-up
  questions; the model can only answer with an array of three strings;
- **status:** `$agent.status()` for the model's download progress, and `wait` with `onWait` so a question asked while
  the model loads waits for it and the page shows the progress;
- **warming:** the preamble is warmed when the app first loads it (`$agent.warm`), so the first question doesn't
  wait for it. The answer's footer shows how many prompt tokens were cached.

```
pb_pages/
  index.ejs              the chat page
  chat/+api.ts           POST /chat {chat?, question}: the answer as server-sent events; GET /chat: status and what the app knows
  chat/suggest/+api.ts   POST /chat/suggest: three follow-up questions (JSON schema)
  _private/agent.ts      loads preamble + context, searches, asks the chat ($agent.chat)
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
`/_/` and pick it in Settings → LLM.

To teach it something else, change `preamble.md` and put your own Markdown files in `_private/context/` (one
`Source: <url>` line at the top of a file links its sections). In dev mode (`--dev`) edits show up on the next question.

## What to expect

On an Apple M1 Max with `qwen3-1.7b`, an answer starts in a second or two and streams at about 40 words a second; the
follow-up questions come a second or two after it ends.
A small x86 server is several times slower; `qwen3-0.6b` is about three times faster than 1.7B and less careful. The
answers are only as good as the three sections it finds, so a question the context doesn't cover gets "I don't know"
or a near miss.
