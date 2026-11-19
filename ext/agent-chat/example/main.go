// Command example is a PocketBase app with stored chats: the agent extension answers, agent-chat keeps the
// conversations. Name a model in pocketone.yaml (`agent: {model: qwen3-0.6b}`) or choose one in Settings → LLM, run
// `go run . serve`, and ask over HTTP as a superuser:
//
//	POST /api/_c/agent.chat/v1/new  {"title": "Support"}            -> {"id": "..."}
//	POST /api/_c/agent.chat/v1/send {"id": "...", "text": "Hello"}  -> {"answer": {"text": "..."}}
//
// In page code the same is $agent.newChat() and $agent.chat(id).send(text).
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"

	"pocketone.dev/ext/agent"
	agentchat "pocketone.dev/ext/agent-chat"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
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
