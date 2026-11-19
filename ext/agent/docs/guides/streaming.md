---
title: Streaming answers
description: "Show an answer as the model writes it: $agent.stream in a page, forwarded to the browser with ctx.res.sse()."
---

# Streaming answers

A model writes its answer a few words at a time. `$agent.stream()` hands you each piece as it's written, so a page can
show the answer as it appears instead of after a wait:

```js
const r = $agent.stream({ prompt: question, maxTokens: 300 }, (text) => {
  // called with each piece, in order; return false to stop the answer there
})
// r is the whole answer, as $agent.complete returns it
```

```go
s, err := agent.Stream(ctx, extkit.Internal, agentv1.CompleteInput{Prompt: question, MaxTokens: 300})
if err != nil {
	return err
}
var r *agentv1.CompleteResult
err = s.Run(ctx, func(e any) error {
	c := e.(agentv1.Chunk) // a piece of text, in order; the last one is Done, with the whole answer
	if c.Done {
		r = c.Result
	}
	return nil // an error stops the answer there
})
```

## In a page

A PocketPages API route forwards the pieces to the browser as server-sent events:

```js
// pb_pages/ask/+api.js
module.exports = {
  post(ctx) {
    const { question } = ctx.req.body()
    const stream = ctx.res.sse()
    try {
      const r = $agent.stream({ prompt: question, maxTokens: 300 }, (text) => stream.send("text", JSON.stringify(text)))
      stream.send("done", JSON.stringify({ usage: r.usage }))
    } catch (err) {
      stream.send("error", JSON.stringify(String(err.message || err)))
    }
    stream.close()
  },
}
```

```go
// POST /ask, in your app's Go code (an app's own route; extensions use contracts instead)
se.Router.POST("/ask", func(e *core.RequestEvent) error {
	var in struct{ Question string }
	if err := e.BindBody(&in); err != nil {
		return e.BadRequestError("send {question}", err)
	}
	s, err := agentv1.Contract.Must(e.App).Stream(e.Request.Context(), extkit.Internal,
		agentv1.CompleteInput{Prompt: in.Question, MaxTokens: 300})
	if err != nil {
		return err
	}
	e.Response.Header().Set("Content-Type", "text/event-stream")
	send := func(event string, v any) error {
		b, _ := json.Marshal(v)
		_, err := fmt.Fprintf(e.Response, "event: %s\ndata: %s\n\n", event, b)
		e.Flush()
		return err
	}
	return s.Run(e.Request.Context(), func(ev any) error {
		if c := ev.(agentv1.Chunk); c.Done {
			return send("done", map[string]any{"usage": c.Result.Usage})
		} else {
			return send("text", c.Text)
		}
	})
})
```

In the browser, read the response as it arrives:

```js
const res = await fetch("/ask", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ question }) })
const reader = res.body.pipeThrough(new TextDecoderStream()).getReader()
let buf = "", answer = ""
for (;;) {
  const { value, done } = await reader.read()
  if (done) break
  buf += value
  let i
  while ((i = buf.indexOf("\n\n")) >= 0) {
    const block = buf.slice(0, i)
    buf = buf.slice(i + 2)
    const event = (block.match(/^event: (.*)$/m) || [])[1]
    const data = (block.match(/^data: (.*)$/m) || [])[1]
    if (event === "text") answer += JSON.parse(data) // show it
  }
}
```

The browser's `EventSource` only makes GET requests, which is why this reads the stream with `fetch`.

## Over HTTP

Outside your app's code, agent/v1's `stream` operation does the same over HTTP: `POST /api/_c/agent/v1/stream` with
the same body as `complete`, as a superuser or with a token that has the `agent` scope. Each piece is an
`event: stream` with `{"text": "..."}`, and the last one is `{"done": true, "result": {...}}`.
