---
owners: ["@benallfree"]
version: 0.0.1
title: "Agent"
group: binary
summary: "A language model for your app's hooks: your own API, or a small model PocketOne runs itself in the same binary."
---

# Agent

Agent gives your app a language model to call from `pb_hooks`: `$agent.complete()`. Use it for the small jobs that
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

**Your own API:** in the dashboard, **Settings → Agent**: the endpoint (the API's base URL with its version, like
`https://api.openai.com/v1` or `http://localhost:11434/v1` for Ollama), the key and the model. **Test** sends a
one-word prompt. The key is never shown again; leave it empty to keep it.

**A model PocketOne runs:** name it in `pocketone.yaml`:

```yaml
agent:
  model: qwen3-1.7b   # qwen3-0.6b, qwen3-1.7b, qwen3-4b, hf:<org>/<repo>/<file>.gguf, or a path to a .gguf file
  threads: 4          # default: half your CPU cores
```

| Model | Download | Memory | Good for |
|---|---|---|---|
| `qwen3-0.6b` | 640 MB | about 1 GB | tags, yes/no, short extraction |
| `qwen3-1.7b` | 1.8 GB | about 2 GB | a good default for a small server |
| `qwen3-4b` | 2.5 GB | about 3 GB | better answers, slower |

The first start downloads the model into your user cache folder (`cache_dir` to change it) and checks its SHA-256;
the app serves meanwhile, and `$agent` answers "still loading" until the model is ready. Your own API, once set,
takes over from it.

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

- `prompt`, or `messages` for a conversation; `system`, `maxTokens`, `temperature`, `stop`.
- `schema`: a JSON schema. The model is told to answer with matching JSON and the parsed value comes back as `data`.
- The answer: `text`, `data`, `model`, `backend` (`app` or `runner`, or `host` on a multisite host), `usage`.
- `$agent.status()` says which model answers and whether it's ready.

`$agent.complete` waits for the answer, like `$http.send`. A model on your CPU writes maybe 5 to 30 words a second,
so keep answers short (`maxTokens`) and do long jobs in a cron, not in a request hook.

It's [agent/v1](../contracts/agent/v1/README.md), also over HTTP (`POST /api/_c/agent/v1/complete` with a superuser
or a token with the `agent` scope), on the command line (`pocketone remote agent complete --prompt "..."`) and as
an MCP tool.

## Integrations

| Extension | How they fit |
|---|---|
| [Settings](../settings/README.md) | Your own API is a settings/v1 section, in the dashboard, the CLI and for agents. |
| [Multisite](../multisite/README.md) | A host whose `agent` section names a model runs it once for every instance and provides [agent/v1](../contracts/agent/v1/README.md) for them; `$agent` in each instance uses it. Each instance gets a share, set by `per_instance` (below). An instance whose admins set their own API in Settings → Agent uses that instead. Today only instances that run as isolates ([Pocker](../pocker/README.md)) reach the host's model. |

On a host:

```yaml
agent:
  model: qwen3-1.7b
  threads: 6
  parallel: 1             # answers written at once; the rest wait their turn
  per_instance:
    concurrency: 2        # calls an instance may have waiting or running at once
    tokens_per_day: 50000 # answer tokens a day (0: no limit)
    max_tokens: 512       # longest answer per call
```

## Example

A PocketBase app that asks its model one question once the model is ready.

```go
// Command example is a PocketBase app with the agent extension that asks its model one question once it's ready. Name
// a model in pocketone.yaml (`agent: {model: qwen3-0.6b}`) or set your own API in Settings → Agent, run
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

## The built-in engine

The model runs in `ext/agent/engine`: Go, no cgo, with assembly for the hot loops on arm64 (Apple silicon, Graviton,
Ampere), where it runs about as fast as llama.cpp on the CPU. On x86 it uses plain Go for now, which is several times
slower. It reads GGUF files (Q4_K, Q6_K, Q8_0, F16) and runs the Qwen3 family. Other models work through your own API
(Ollama, llama-server).

## License

MIT
