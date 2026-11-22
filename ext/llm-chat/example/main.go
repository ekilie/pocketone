// Command example is a PocketBase app with stored chats: the llm extension answers, llm-chat keeps the
// conversations. Add a provider and a text default in Settings → LLM, run `go run . serve`, and ask over HTTP as a
// superuser:
//
//	POST /api/_c/llm.chat/v1/new  {"title": "Support"}            -> {"id": "..."}
//	POST /api/_c/llm.chat/v1/send {"id": "...", "text": "Hello"}  -> {"answer": {"text": "..."}}
//
// In page code the same is $llm.chat.new() and $llm.chat.open(id).send(text).
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"

	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/llm"
	llmchat "pocketone.dev/ext/llm-chat"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app)
	if _, err := extkit.Setup(app, []extkit.Extension{llm.New(), llmchat.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
