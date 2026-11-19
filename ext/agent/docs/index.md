---
title: AI in your app
section: New in PocketOne
description: "$agent gives your pages and Go code a language model: label records, pull JSON out of text, summarize, answer questions. Built in, on your own CPU, or through any OpenAI-compatible API."
---

# AI in your app

**Your app can ask a language model.** `$agent.complete()` sends a prompt and gives you the answer, from your pages'
code; Go code calls the same thing through the agent/v1 contract:

```ts
const r: AgentAnswer = $agent.complete({ prompt: "Summarize in one sentence: " + record.getString("body"), maxTokens: 60 })
// r.text is the summary
```

```go
r, err := agentv1.Contract.Must(app).Complete(ctx, extkit.Internal, agentv1.CompleteInput{
	Prompt: "Summarize in one sentence: " + record.GetString("body"), MaxTokens: 60})
// r.Text is the summary
```

The examples show each thing in TypeScript, as [PocketPages](../../pages/docs/overview.md) page code (`+api.ts`,
`+page.ts`), and in Go: pick one above any example and the pages remember it. The Go examples call the contract package
(`agentv1`, from `github.com/pockethost/pocketone/ext/contracts/agent/v1`) with a `context.Context` named `ctx`;
`agent` stands for `agentv1.Contract.Must(app)`.

Use it for the small jobs that come up around your data: label a support ticket, pull an address out of an email,
summarize a long post, write a title or an alt text, answer a question about your docs.

It's built into `pocketone`. The model can be one PocketOne runs itself, on your server's CPU with nothing else to
install, or any OpenAI-compatible API (OpenAI, OpenRouter, Groq, or Ollama on your own machine). Your code is the same
either way.

## Try it in two minutes

1. Pick a model. In the dashboard, open **Settings → LLM**, choose **Built-in** and `qwen3-1.7b`, and save. The first
   time, PocketOne downloads it (1.8 GB) and shows the progress. Or add it to `pocketone.yaml`:

   ```yaml
   agent:
     model: qwen3-1.7b
   ```

2. Label support tickets as they come in. Make a `tickets` collection with two text fields, `body` and `label`, and a
   route that saves each new ticket with the label the model picks:

   ```ts
   // pb_pages/tickets/+api.ts: POST /tickets {"body": "..."}
   export function post(ctx: any) {
     const { body } = ctx.req.body()
     const r: AgentAnswer = $agent.complete({
       system: "Label the support ticket.",
       prompt: body,
       schema: { type: "object", properties: { label: { enum: ["bug", "billing", "question"] } }, required: ["label"] },
       maxTokens: 30,
     })
     const ticket = new Record(ctx.app.findCollectionByNameOrId("tickets"))
     ticket.set("body", body)
     ticket.set("label", r.data.label)
     ctx.app.save(ticket)
     return ctx.res.json(201, { id: ticket.id, label: r.data.label })
   }
   ```

   ```go
   // POST /tickets {"body": "..."}, in your app's Go code
   se.Router.POST("/tickets", func(e *core.RequestEvent) error {
   	var in struct{ Body string }
   	if err := e.BindBody(&in); err != nil {
   		return e.BadRequestError("send {body}", err)
   	}
   	r, err := agentv1.Contract.Must(e.App).Complete(e.Request.Context(), extkit.Internal, agentv1.CompleteInput{
   		System:    "Label the support ticket.",
   		Prompt:    in.Body,
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
   	col, err := e.App.FindCollectionByNameOrId("tickets")
   	if err != nil {
   		return err
   	}
   	ticket := core.NewRecord(col)
   	ticket.Set("body", in.Body)
   	ticket.Set("label", out.Label)
   	if err := e.App.Save(ticket); err != nil {
   		return err
   	}
   	return e.JSON(201, map[string]string{"id": ticket.Id, "label": out.Label})
   })
   ```

3. Send it a ticket: `curl -X POST localhost:8090/tickets -H "Content-Type: application/json" -d '{"body": "I was charged twice this month"}'`.
   Its `label` is `billing`.

The `schema` makes the model answer with JSON in exactly that shape, and `r.data` is the parsed value: see
[Structured answers](guides/structured.md).

## What's next

- [Choosing a model](choosing.md): built in or your own API, which model, and how fast each is.
- [Writing a preamble](guides/preamble.md): the system prompt that sets how it answers.
- [Remembering conversations](guides/conversations.md): keep chats, summarize the old turns, recall past chats.
- [Structured answers](guides/structured.md): JSON you can rely on, even from a small model.
- [Streaming answers](guides/streaming.md): show an answer as it's written.
- [Your own knowledge](guides/context.md): answer questions about your docs or data.
- [On a multisite host](hosting.md): one model shared by every app on a server.
- [Prompt caching](prompt-caching.md): how the model skips re-reading what your prompts repeat, and how to order them
  for it.

The API is [agent/v1](../../contracts/agent/v1/README.md); the extension's reference is [its README](../README.md).
