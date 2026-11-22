# Docs Chat

A chat app about PocketOne, written as a PocketPages app with no Go: `pb_pages` and the stock `pocketone` binary. It
runs live at
[learn.pocketone.dev](https://learn.pocketone.dev). It shows `$llm` from page code:

- **answering from both sets of docs:** [`_private/llm.ts`](pb_pages/_private/llm.ts) loads (off the VM, every 10
  minutes) PocketOne's user docs from pocketone.dev's `llms.txt` and `llms-full.txt` (the pages under Get started, New
  in PocketOne and Guides, plus PocketPages, not the extension and contract reference) and PocketBase's own docs from
  the Markdown copy PocketOne keeps of pocketbase.io/docs
  ([`agent/skills/pocketone/reference/pocketbase/`](../../../../agent/skills/pocketone/reference/pocketbase/README.md)).
  It splits them into sections, and for each question picks the three that match best (BM25, plus a few synonyms:
  people say "apps", the docs say "instances"), each labelled "PocketBase docs" or "PocketOne docs" and linked to its
  site, after a preamble ([`preamble.md`](pb_pages/_private/preamble.md)) that says how the two fit together;
- **a stored chat, answered as it's written:** `$llm.chat.new()` once, then
  `await $llm.chat.open(id, { model, system }).sendAsync(question, { context, onText })` each turn (the llm-chat
  extension): the chat keeps the conversation (the page holds only its id), sends the preamble, the turns so far and
  the question with its excerpts after it, and keeps the question as sent, so each turn's prompt starts like the last
  one (providers bill a repeated start cheaper); [`chat/+api.ts`](pb_pages/chat/+api.ts) forwards each piece to the
  browser with `ctx.res.sse()`;
- **structured output:** a second call, `await $llm.text.completeAsync({ schema })`, suggests three follow-up
  questions as JSON in the shape the schema asks for;
- **async throughout:** every `$llm` call is awaited in an async handler, so the model works off the app's JS VM and
  the app keeps serving other requests while an answer streams.

```
pb_pages/
  index.ejs              the chat page
  chat/+api.ts           POST /chat {chat?, question}: the answer as server-sent events; GET /chat: the model and what the app knows
  chat/suggest/+api.ts   POST /chat/suggest: three follow-up questions (JSON schema)
  _private/llm.ts        the model's name, fetches the docs, searches, asks the chat ($llm.chat)
  _private/preamble.md   who the assistant is and how it answers
```

## Run it

```bash
cd ext/llm/examples/chat
pocketone superuser upsert admin@example.com password --dir=pb_data
pocketone serve --dir=pb_data
```

Sign in at http://127.0.0.1:8090/_/ and add a provider in **Settings → LLM** (the app answers with `vultr/glm-5.2`, so
a Vultr provider named `vultr`; or see below). Then open http://127.0.0.1:8090.

To answer with another model, set `CHAT_MODEL` to its `provider/model` when you start the app, for example
`CHAT_MODEL=or/openai/gpt-4o-mini pocketone serve --dir=pb_data` with an OpenRouter provider named `or`, or change
`MODEL` in [`_private/llm.ts`](pb_pages/_private/llm.ts).

It reads PocketBase's docs from that folder on PocketOne's `main` (GitHub's raw URL); set `PB_DOCS` to another URL or
to a local `llms-full.txt`. It answers from `https://pocketone.dev`; set `DOCS_URL` to answer from another copy of the
site (a local one at
`http://127.0.0.1:8090`, say). Any site with the same `llms.txt` / `llms-full.txt` shape works, so pointing it at your
own docs means changing `DOCS_URL` and `SECTIONS` in `_private/llm.ts`, and `preamble.md`.

## What to expect

With `vultr/glm-5.2`, an answer starts in about a second and streams quickly; the follow-up questions come a second
after it ends. Each question costs a fraction of a cent on your provider account (the footer under each answer shows
the tokens it used). The answers are only as good as the three sections it finds, so a question the context doesn't
cover gets "I don't know" or a near miss.
