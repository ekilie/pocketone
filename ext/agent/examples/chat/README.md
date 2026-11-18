# Docs Chat

A small chat app on [agent/v1](../../../contracts/agent/v1/README.md): ask questions about whatever an `llms-full.txt`
describes, answered by a model PocketOne runs on your CPU (or your own API).

For each question it picks the three passages of the knowledge that match best (BM25 over the pages' sections) and
gives only those to the model, with the last few turns of the conversation. A small model then answers in a few
seconds instead of reading the whole file every time, and each answer lists the pages it came from.

## Run it

```bash
cd ext/agent/examples/chat
go run . serve
```

Open http://127.0.0.1:8090. The first start downloads the model named in [pocketone.yaml](pocketone.yaml)
(`qwen3-1.7b`, 1.8 GB) into your user cache folder; the page shows the progress. Log in to the dashboard at `/_/` with
`admin@example.com` / `password` to set your own API instead (Settings → Agent).

## Knowledge

`--knowledge` takes `llms-full.txt` files, URLs or folders of Markdown pages, comma-separated. The default is
PocketOne's docs, `https://pocketone.dev/llms-full.txt`.

```bash
go run . serve --knowledge=https://pocketone.dev/llms-full.txt
go run . serve --knowledge=/path/to/llms-full.txt,/path/to/more-docs/   # a folder: every .md, titled by its front matter
```

## What to expect

On an Apple M1 Max with `qwen3-1.7b`, answers take 3 to 7 seconds. On a small x86 server expect several times that,
or use `qwen3-0.6b`. Answers come only from the passages it found, so a question the docs don't cover gets "I don't
know" or a near miss.

There's no streaming yet (agent/v1 answers in one piece), so the page shows a timer while it waits.
