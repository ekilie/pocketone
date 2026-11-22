// Command example is a PocketBase app with the llm extension. Add a provider named "vultr" in Settings → LLM (or
// change the model below), run `go run . serve`, and watch the log for "llm: answer".
package main

import (
	"context"
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	textv1 "pocketone.dev/ext/contracts/llm/text/v1"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/llm"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app)
	if _, err := extkit.Setup(app, []extkit.Extension{llm.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		go func() {
			res, err := textv1.Contract.Must(app).Complete(context.Background(), extkit.Internal, textv1.CompleteInput{
				Model: "vultr/glm-5.2", Prompt: "In one sentence: what is PocketBase?", MaxTokens: 200, Reasoning: "off"})
			app.Logger().Info("llm: answer", "text", res.Text, "model", res.Model, "error", err)
		}()
		return e.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
