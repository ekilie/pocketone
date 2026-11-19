---
owners: ["@benallfree"]
version: 0.0.1
title: "Agent Chat"
group: binary
summary: "Stored conversations with your app's language model: $agent.chat(id).send(text), with history and summaries."
---

# Agent Chat

Agent Chat keeps conversations with your app's language model. Start a chat, keep its id, and send it questions:
it stores every turn, sends the model the recent ones with each new question, folds older ones into a summary so long
chats stay fast, and with the built-in model keeps each chat's state between turns, so a turn reads only what's new.

```js
const id = $agent.newChat({ meta: { user: e.auth.id } })
const answer = $agent.chat(id, { system: preamble }).send("How do I export my notes?")
```

It needs the [Agent](../agent/README.md) extension (agent/v1), which answers. Chats aren't tied to users: your app
keeps the id (on its own records, or in its frontend) and may put anything in a chat's `meta` to find it again.

## Install

Agent Chat isn't part of the stock `pocketone` binary yet. Add it to your own Go build, next to Agent:

```go
extkit.Setup(app, []extkit.Extension{agent.New(), agentchat.New()}, nil, nil)
```

On a build from the standard set: `set.Add(agentchat.New())`. The module is
`github.com/pockethost/pocketone/ext/agent-chat`. It creates its two collections on the first start, with its own
migrations (`pocketone extensions migrations` lists them).

## Configure

There's nothing to configure. Each send can say how much of the chat to send (below). Without the extension, its
collections and their data stay; `pocketone extensions migrate down agent-chat` removes them (data included) while
it's still in the build.

## Use

`$agent.newChat({ title?, meta? })` returns a new chat's id. `$agent.chat(id, options)` returns the chat:

| Method | What it does |
|---|---|
| `send(text, { onText?, onWait?, schema? })` | Asks, saves both turns, returns agent/v1's answer (`text`, `usage`, ...). With `onText`, streams the answer to it as it's written. |
| `send.async(text, { schema? })` | The same in an async hook, waiting for a model that's still loading. |
| `history({ limit?, before? })` | The chat's messages, oldest first: `{ id, role, content, created }`. |
| `summary()` | What the older messages were folded into. |
| `title(text?)`, `meta(object?)` | Read, or set and return. |
| `forget()` | Deletes the chat, its messages and the model's state for it. |

Options for `$agent.chat(id, ...)`, used by every send:

| Option | Default | What it does |
|---|---|---|
| `system` | | The preamble, first on every turn ([Writing a preamble](../agent/docs/guides/preamble.md)). |
| `context` | | Text, or a function of the question returning text, sent after the question and not saved: the passages that answer it. |
| `window` | 12 | Messages sent before the summary takes over. |
| `summarizeEvery` | 6 | Once that many messages have piled up past the window, the oldest are folded into the summary (a second, short call). |
| `maxTokens`, `temperature`, `wait` | agent's | As in `$agent.complete`. |

`$agent.chats.list({ meta?, limit?, offset? })` lists chats, most recently updated first; `meta` keeps those whose
meta has each of its top-level keys with that value. `$agent.chats.forget({ meta })` deletes every match and returns
how many.

The chats are in `_agent_chat_chats` and `_agent_chat_messages`, superuser-only: your routes decide what users see.
A typical route checks that the chat's `meta.user` is the caller before sending:

```js
routerAdd("POST", "/api/ask/{chat}", (e) => {
  const chat = $agent.chat(e.request.pathValue("chat"), { system: preamble })
  if (chat.meta().user !== e.auth?.id) throw new NotFoundError()
  return e.json(200, chat.send(e.requestInfo().body.question))
}, $apis.requireAuth())
```

**How a send is built:** the system text, then the summary (when there is one), then every message since the last
fold, then the question with its context. That start stays the same from one turn to the next until the next fold,
which is what the built-in model's prompt cache and its per-chat thread need to skip re-reading the conversation.

## Integrations

| With | What happens |
|---|---|
| [Agent](../agent/README.md) | Required: it answers (agent/v1), with the chat's id as the thread. Its HTTP and MCP API is the [agent.chat/v1](../contracts/agent/chat/v1/README.md) contract. |
| [Multisite](../multisite/README.md) | Each instance keeps its own chats in its own database; answers come from the instance's agent (its own API, or the host's shared model). |

## Example

```go
// Command example is a PocketBase app with stored chats: the agent extension answers, agent-chat keeps the
// conversations. Name a model in pocketone.yaml (`agent: {model: qwen3-0.6b}`) or choose one in Settings → LLM, run
// `go run . serve`, and ask over HTTP as a superuser:
//
//	POST /api/_c/agent.chat/v1/new  {"title": "Support"}            -> {"id": "..."}
//	POST /api/_c/agent.chat/v1/send {"id": "...", "text": "Hello"}  -> {"answer": {"text": "..."}}
//
// In pb_hooks the same is $agent.newChat() and $agent.chat(id).send(text).
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"

	"github.com/pockethost/pocketone/ext/agent"
	agentchat "github.com/pockethost/pocketone/ext/agent-chat"
	"github.com/pockethost/pocketone/ext/devusers"
	"github.com/pockethost/pocketone/ext/extkit"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app)
	if _, err := extkit.Setup(app, []extkit.Extension{agent.New(), agentchat.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## License

MIT
