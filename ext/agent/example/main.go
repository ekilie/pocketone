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

	"pocketone.dev/ext/agent"
	agentv1 "pocketone.dev/ext/contracts/agent/v1"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
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
