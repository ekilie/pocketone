// Command chat is a small chat app on agent/v1: ask questions about whatever an llms-full.txt describes (PocketOne's
// docs by default), answered by the model PocketOne runs itself or by your own API.
//
// For each question it picks the few passages of the knowledge that match best (BM25) and gives only those to the
// model, so a small model on a CPU answers in seconds instead of reading the whole file every time.
//
//	go run . serve                                    # PocketOne's docs, the model named in pocketone.yaml
//	go run . serve --knowledge=pockethost.txt         # any llms-full.txt, file or URL; comma-separate several
//
// Then open http://127.0.0.1:8090.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"github.com/pockethost/pocketone/ext/agent"
	"github.com/pockethost/pocketone/ext/config"
	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	"github.com/pockethost/pocketone/ext/devusers"
	"github.com/pockethost/pocketone/ext/extkit"
)

//go:embed index.html
var page embed.FS

func main() {
	if _, err := os.Stat("pocketone.yaml"); err == nil {
		if _, err := config.Load(config.Options{ConfigPath: "pocketone.yaml"}); err != nil {
			log.Fatal(err)
		}
	}
	app := pocketbase.New()
	devusers.Register(app)
	var sources string
	app.RootCmd.PersistentFlags().StringVar(&sources, "knowledge", "https://pocketone.dev/llms-full.txt",
		"llms-full.txt files or URLs the assistant answers from (comma-separated)")
	if _, err := extkit.Setup(app, []extkit.Extension{agent.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}

	var kb atomic.Pointer[knowledge]
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		go func() {
			t := time.Now()
			k, err := loadKnowledge(strings.Split(sources, ","))
			if err != nil {
				app.Logger().Error("chat: knowledge", "error", err)
				return
			}
			kb.Store(k)
			app.Logger().Info("chat: knowledge loaded", "passages", len(k.chunks), "took", time.Since(t).Round(time.Millisecond))
		}()

		se.Router.GET("/", func(e *core.RequestEvent) error {
			b, _ := page.ReadFile("index.html")
			return e.HTML(http.StatusOK, string(b))
		})
		se.Router.GET("/api/chat/status", func(e *core.RequestEvent) error {
			st, _ := agentv1.Contract.Must(app).Status(e.Request.Context(), extkit.Internal)
			k := kb.Load()
			passages := 0
			if k != nil {
				passages = len(k.chunks)
			}
			return e.JSON(http.StatusOK, map[string]any{"agent": st, "passages": passages, "knowledge": sources})
		})
		se.Router.POST("/api/chat", func(e *core.RequestEvent) error {
			var in struct {
				Messages []agentv1.Message `json:"messages"`
			}
			if err := e.BindBody(&in); err != nil || len(in.Messages) == 0 {
				return e.BadRequestError("send {messages: [{role, content}, ...]}", err)
			}
			return chat(e, app, kb.Load(), in.Messages)
		})
		return se.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

const intro = `You are a helpful assistant. Answer the user's question using the documentation excerpts below. ` +
	`Be brief and concrete: a few sentences or a short list, with commands or settings exactly as written. ` +
	`If the excerpts don't answer the question, say you don't know rather than guessing.`

// chat answers the conversation's last question from the best-matching passages.
func chat(e *core.RequestEvent, app core.App, k *knowledge, msgs []agentv1.Message) error {
	if k == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "the knowledge is still loading"})
	}
	if len(msgs) > 6 { // the last few turns are enough, and every token is read on a CPU
		msgs = msgs[len(msgs)-6:]
	}
	// search with the question, plus the one before it so follow-ups ("and how do I undo that?") still find their topic
	var query []string
	for i := len(msgs) - 1; i >= 0 && len(query) < 2; i-- {
		if msgs[i].Role == "user" {
			query = append(query, msgs[i].Content)
		}
	}
	hits := k.search(strings.Join(query, " "), 3)
	var sys strings.Builder
	sys.WriteString(intro)
	sys.WriteString("\n\nDocumentation excerpts:\n")
	sources := []map[string]string{}
	for i, h := range hits {
		fmt.Fprintf(&sys, "\n[%d] %s\n%s\n", i+1, h.Title, h.Text)
		sources = append(sources, map[string]string{"title": h.Title, "url": h.Source})
	}
	ctx, cancel := context.WithTimeout(e.Request.Context(), 3*time.Minute)
	defer cancel()
	t := time.Now()
	temp := 0.3
	res, err := agentv1.Contract.Must(app).Complete(ctx, extkit.Internal, agentv1.CompleteInput{
		System: sys.String(), Messages: msgs, MaxTokens: 350, Temperature: &temp,
	})
	if err != nil {
		var ce *extkit.Error
		status := http.StatusInternalServerError
		if errors.As(err, &ce) && ce.Status != 0 {
			status = ce.Status
		}
		return e.JSON(status, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{
		"answer": strings.TrimSpace(res.Text), "sources": sources, "model": res.Model, "backend": res.Backend,
		"usage": res.Usage, "ms": time.Since(t).Milliseconds(),
	})
}
