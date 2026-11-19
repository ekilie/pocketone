---
owners: ["@benallfree"]
version: 0.0.1
title: "Agent"
group: binary
summary: "A language model for your app's hooks: your own API, or a small model PocketOne runs itself in the same binary."
---

# Agent

Agent gives your app a language model to call: `$agent.complete()` from `pb_hooks` and pages, or the agent/v1
contract from Go (`agentv1.Contract.Must(app).Complete(...)`). Use it for the small jobs that
come up around data: label a support ticket, pull a JSON object out of an email, summarize a record, write a title or
an alt text.

The model can come from two places, and your code doesn't change between them:

- **Your own API.** Any OpenAI-compatible endpoint: OpenAI, OpenRouter, Groq, or a model you run yourself with
  Ollama or llama-server.
- **A model PocketOne runs itself.** Name one in `pocketone.yaml` and PocketOne downloads it once, checks it and runs
  it on your server's CPU, in the same binary (no Python, no GPU, no other program to install). These are small
  models: good at short, focused jobs, not at long conversations.

## Install

Agent is built into `pocketone`. In your own Go build:

```go
extkit.Setup(app, []extkit.Extension{agent.New()}, nil, nil)
```

## Configure

In the dashboard, **Settings → LLM** (under System) picks what answers:

- **Off:** `$agent` answers with an error.
- **Built-in:** a model PocketOne runs, chosen from the list below. Choosing another one downloads it (the page shows
  the progress) and unloads the one before.
- **API:** your own OpenAI-compatible endpoint (the API's base URL with its version, like `https://api.openai.com/v1`,
  or `http://localhost:11434/v1` for Ollama), its key and the model. **Test** sends a one-word prompt. The key is never
  shown again; leave it empty to keep it (a new endpoint needs the key entered again).

Until you choose there, `pocketone.yaml` decides: the model it names runs built in, and without one the LLM is off.
**Reset to pocketone.yaml** on the same page goes back to it. To name the model there:

```yaml
agent:
  model: qwen3-1.7b   # qwen3-0.6b, qwen3-1.7b, qwen3-4b, hf:<org>/<repo>/<file>.gguf, or a path to a .gguf file
  threads: 4          # default: half your CPU cores
  prompt_cache_mb: 256 # keeps the start of recent prompts so a repeated one isn't read again (-1: off)
  thread_memory_mb: 512 # conversations' state between calls, in memory (the thread option)
  thread_cache_gb: 2    # and on disk, in pb_cache/agent/threads (-1: off)
```

| Model | Download | Memory | Good for |
|---|---|---|---|
| `qwen3-0.6b` | 640 MB | about 1 GB | tags, yes/no, short extraction |
| `qwen3-1.7b` | 1.8 GB | about 2 GB | a good default for a small server |
| `qwen3-4b` | 2.5 GB | about 3 GB | better answers, slower |

The first start downloads the model into your user cache folder (`cache_dir` to change it) and checks its SHA-256;
the app serves meanwhile, and `$agent` answers "still loading" until the model is ready. Nothing downloads for a
command (`superuser`, `migrate`); only `serve` starts the model.

## Use

```js
// pb_hooks/tickets.pb.js
onRecordCreate((e) => {
  const r = $agent.complete({
    system: "Label the support ticket.",
    prompt: e.record.get("body"),
    schema: { type: "object", properties: { label: { enum: ["bug", "billing", "question"] } }, required: ["label"] },
    maxTokens: 30,
  })
  e.record.set("label", r.data.label)
  e.next()
}, "tickets")
```

```go
// in your Go app
app.OnRecordCreate("tickets").BindFunc(func(e *core.RecordEvent) error {
	r, err := agentv1.Contract.Must(e.App).Complete(e.Context, extkit.Internal, agentv1.CompleteInput{
		System:    "Label the support ticket.",
		Prompt:    e.Record.GetString("body"),
		Schema:    json.RawMessage(`{"type":"object","properties":{"label":{"enum":["bug","billing","question"]}},"required":["label"]}`),
		MaxTokens: 30,
	})
	if err != nil {
		return err
	}
	var out struct{ Label string }
	if err := json.Unmarshal(r.Data, &out); err != nil {
		return err
	}
	e.Record.Set("label", out.Label)
	return e.Next()
})
```

In Go, every option below is a field of `agentv1.CompleteInput` (`Prompt`, `Messages`, `Schema`, `Thread`, `Wait`, ...)
and every `$agent` call is a method of `agentv1.Agent` (`Complete`, `Stream`, `Status`, `Warm`, `Count`, `Threads`);
pass `extkit.Internal` to call with the app's own rights. The `.async` forms are JS only: Go passes `Wait` instead.

- `prompt`, or `messages` for a conversation; `system`, `maxTokens`, `temperature`, `stop`.
- `schema`: a JSON schema; the parsed answer comes back as `data`. With a model PocketOne runs, the answer can only be JSON
  matching it: the model is kept to it token by token (type, enum, const, properties, required, items, minItems,
  maxItems, anyOf), so even the smallest model fills in the schema instead of describing it. Your own API gets it as
  `response_format`, which most providers honor.
- The answer: `text`, `data`, `model`, `backend` (`app` or `runner`, or `host` on a multisite host), `usage`.
- `$agent.stream(opts, onText)` answers the same way, calling `onText(piece)` as the answer is written (return `false`
  to stop); forward the pieces to the browser with `ctx.res.sse()` in a page.
- `$agent.status()` says which model answers and whether it's ready.
- `thread: "<id>"` keeps a conversation's state between calls: the next call that sends the same conversation plus a
  new question reads only the new question ([Threads](#warming-and-threads)).
- `$agent.warm({ system })` reads a preamble ahead of the calls that start with it, and `$agent.count(text)` counts
  tokens, for budgeting what you send.
- For stored chats with history and summaries, add [Agent Chat](../agent-chat/README.md): `$agent.chat(id).send(text)`.

`$agent.complete` waits for the answer, like `$http.send`. A model on your CPU writes maybe 5 to 30 words a second,
so keep answers short (`maxTokens`) and do long jobs in a cron, not in a request hook.

While the model downloads or loads, `$agent.complete` fails at once with `not_ready`. In an async hook,
`await $agent.complete.async(opts)` waits for it instead (up to `opts.wait`, default `timeout`), and
`$agent.stream(opts, onText, { onWait })` with `wait` calls `onWait(status)` while it waits, to show progress
([When the model is loading](docs/guides/waiting.md)).

It's [agent/v1](../contracts/agent/v1/README.md), also over HTTP (`POST /api/_c/agent/v1/complete`, or
`/api/_c/agent/v1/stream` for server-sent events, with a superuser or a token with the `agent` scope), on the command line (`pocketone remote agent complete --prompt "..."`) and as
an MCP tool.

## Integrations

| Extension | How they fit |
|---|---|
| [Settings](../settings/README.md) | Your own API is a settings/v1 section, in the dashboard, the CLI and for agents. |
| [Agent Chat](../agent-chat/README.md) | Stored conversations on top of agent/v1: `$agent.newChat()`, `$agent.chat(id).send(text)`, `$agent.chats`. Agent binds those methods whenever a provider of agent.chat/v1 runs. |
| [Multisite](../multisite/README.md) | A host whose `agent` section names a model runs it once for every instance and provides [agent/v1](../contracts/agent/v1/README.md) for them; `$agent` in each instance uses it. Each instance gets a share, set by `per_instance` (below). Each instance's Settings → LLM offers Off, the host's model or its own API; which model runs built in is the host's call. Today only instances that run as isolates ([Pocker](../pocker/README.md)) reach the host's model. |

On a host:

```yaml
agent:
  model: qwen3-1.7b
  threads: 6
  parallel: 1             # answers written at once; the rest wait their turn
  per_instance:
    concurrency: 2        # calls an instance may have waiting or running at once
    max_tokens: 512       # longest answer per call
    pinned_mb: 64         # memory an instance's warmed prompts may hold
    thread_cache_gb: 2.5  # disk for an instance's conversation state, in its own pb_cache
```

## Example

A PocketBase app that asks its model one question once the model is ready.

```go
// Command example is a PocketBase app with the agent extension that asks its model one question once it's ready. Name
// a model in pocketone.yaml (`agent: {model: qwen3-0.6b}`) or choose one in Settings → LLM, run
// `go run . serve`, and watch the log for "agent: answer".
package main

import (
	"context"
	"log"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"github.com/pockethost/pocketone/ext/agent"
	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	"github.com/pockethost/pocketone/ext/devusers"
	"github.com/pockethost/pocketone/ext/extkit"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app)
	if _, err := extkit.Setup(app, []extkit.Extension{agent.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		go func() {
			a := agentv1.Contract.Must(app)
			for {
				st, _ := a.Status(context.Background(), extkit.Internal)
				if st.Ready || st.Backend == "" {
					break
				}
				app.Logger().Info("agent: waiting", "status", st.Message)
				time.Sleep(5 * time.Second)
			}
			res, err := a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{
				Prompt: "In one sentence: what is PocketBase?", MaxTokens: 60})
			app.Logger().Info("agent: answer", "text", res.Text, "backend", res.Backend, "error", err)
		}()
		return e.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## Prompt caching

The built-in model keeps the start of recent prompts: the internal state it computed while reading them (the
attention keys and values). When a new prompt starts with exactly the same text, that state is copied in and only the
rest is read. Reading the prompt is most of the wait on a CPU, so a repeated preamble or a chat's earlier turns cost
almost nothing the second time. The answer is the same as reading the whole prompt again: the cached state is exactly
what the model would recompute.

```yaml
agent:
  prompt_cache_mb: 256   # default; -1 turns it off
```

- **Memory:** each cached token takes about 230 KB for `qwen3-0.6b` and `qwen3-1.7b`, and 290 KB for `qwen3-4b`, so
  the default 256 MB holds about 1,100 tokens (870 for 4B). The least recently used entries make room for new ones.
- **What it keeps:** the start of each prompt, as much as fits. A prompt reuses the entry it shares the longest start
  with, if that's at least 32 tokens; its last token is always read again.
- **On a multisite host:** each instance's prompts only reuse that instance's entries, so timing can't tell an instance
  what another one sent. The budget is the host's, shared by all of them.
- **Seeing it work:** `usage.cachedTokens` in each answer is how many prompt tokens came from the cache. For your own
  API it's the provider's own figure, when it reports one (OpenAI's `prompt_tokens_details.cached_tokens`).

Measured on an Apple M1 Max, a second prompt sharing a ~160-token preamble with the first: `qwen3-0.6b` read it in
56 ms instead of 473 ms, `qwen3-1.7b` in 0.13 s instead of 2.2 s.

The cache matches only an identical start, so order a prompt from what never changes to what changes most: the
preamble, then the conversation so far, then the new question with the excerpts picked for it after it. See
[Prompt caching](docs/prompt-caching.md) in the docs.

## Warming and threads

Two more ways to skip reading the same text, both for the built-in model (your own API ignores them; providers cache
prompts themselves):

- **Warm a preamble:** `$agent.warm({ system: preamble })` reads it now, in the background, and keeps it in the prompt
  cache until `forget()`, so the first question after a start doesn't wait for it and other prompts can't push it out.
  It returns `{ id, warmed, forget() }` at once; `await $agent.warm.async(...)` resolves once it's warm. The same text
  always gets the same id. On a multisite host each instance's warms are capped by `per_instance.pinned_mb`.
- **Threads:** `thread: chatId` on a call keeps the conversation's whole state afterwards, the model's own answer
  included, and the next call continues from it as far as its messages start the same way. Send the earlier turns
  exactly as before (excerpts and all) and each turn reads only the new question. States are kept in memory
  (`thread_memory_mb`) and on disk at half precision in `pb_cache/agent/threads` (`thread_cache_gb`), least recently
  used first: a thread survives a restart, and reading it back takes a fraction of a second where re-reading a long
  chat takes many. `$agent.threads.list()` and `.forget(id)` manage them. On a host each instance's threads are in its
  own `pb_cache`, within `per_instance.thread_cache_gb`.

A thread is a cache, not storage: your app still sends the conversation. [Agent Chat](../agent-chat/README.md) does
all of this for you.

## The built-in engine

The model runs in `ext/agent/engine`: Go, no cgo, with assembly for the hot loops on arm64 (NEON) and x86 (AVX2), where
it matches or beats llama.cpp-based servers on the CPU for short jobs. Other CPUs fall back to plain Go, which is many
times slower. It reads GGUF files (Q4_K, Q6_K, Q8_0, F16) and runs the Qwen3 family. Other models work through your own
API (Ollama, llama-server).

[examples/chat](examples/chat/README.md) is a PocketPages app (no Go) that arms `$agent` with a preamble and context
files read from disk and chats about PocketOne.

## License

MIT
