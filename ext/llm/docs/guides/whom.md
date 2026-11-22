---
title: Whom a call is for
description: "$llm.for(ctx) counts a page's model calls for its signed-in user or its guest, so per-user and per-guest limits apply; how guests are told apart, and how to handle a refused call."
---

# Whom a call is for

[Limits](limits.md) on each user, each guest and all guests together need to know whom each call is for. Page code says
so with one change: call the model through `$llm.for(ctx)` instead of `$llm`.

```ts
// pb_pages/ask/+api.ts
export async function post(ctx: pages.Context) {
  const { question } = ctx.req.body()
  const r: LlmTextAnswer = await $llm.for(ctx).text.completeAsync({ model: "vultr/glm-5.3", prompt: question })
  return ctx.res.json({ answer: r.text })
}
```

`$llm.for()` takes the request you're answering: a page's `ctx`, or a `pb_hooks` route's `e`. It returns the same API as
`$llm`, every kind, chats and streams included, with each call counted for that request:

- **its signed-in user**, when the request carries an auth record (counted as `user:<collection>/<id>`);
- else **a guest**, told apart by a cookie (below).

```js
// pb_hooks/ask.pb.js
routerAdd("POST", "/api/ask", async (e) => {
  const { question } = e.requestInfo().body
  const r = await $llm.for(e).text.completeAsync({ model: "vultr/glm-5.3", prompt: question })
  return e.json(200, { answer: r.text })
})
```

Keep the object for the whole request when you make several calls, so they all count for the same person:

```ts
const llm = $llm.for(ctx)
const chat = llm.chat.open(chatId, { model: "vultr/glm-5.3" })
const r = await chat.sendAsync(question)
const more = await llm.text.completeAsync({ model: "vultr/glm-5.3", prompt: `Three follow-up questions to: ${question}` })
```

## Calls that aren't for anyone

Plain `$llm` counts toward limits on **the whole app** only. Use it for work that isn't on someone's behalf: a cron,
a hook that tags new records, a migration. Those calls are recorded with no subject, never count as a guest's, and are
never refused for a missing guest total.

```js
// pb_hooks/tags.pb.js: tags every new post; counted toward the whole app only
onRecordAfterCreateSuccess(async (e) => {
  const r = await $llm.text.completeAsync({ model: "vultr/glm-5.3", prompt: "Three tags for: " + e.record.get("title") })
  e.record.set("tags", r.text)
  $app.save(e.record)
  e.next()
}, "posts")
```

Calls over HTTP and MCP (`/api/_c/llm.text/v1/...`) come from superusers and API tokens, never from visitors, so they
also count toward the whole app only.

## Guests

A visitor who isn't signed in is a guest. Guests are told apart by a cookie, `pocketone_llm_guest`, which the first
`$llm.for()` call for a visitor sets on the response:

- It holds a random id and a signature, so a visitor can't pick someone else's id. The signing key is made the first
  time it's needed and kept in the app's [key-value store](/docs/extensions/kv) at the private name `_llm/guest-key`.
  Deleting it gives every guest a new id.
- It lasts a year, is `HttpOnly` (page JavaScript can't read it), `SameSite=Lax`, and `Secure` when the app is served
  over HTTPS.
- Until the browser sends it back, the visitor counts by IP address (`guest:ip:<address>`). The first call of a visit
  is counted that way, and so is every call from a client that never keeps cookies, so dropping cookies doesn't get a
  fresh allowance on every call.

A visitor who clears their cookies comes back as a new guest. That's why guest calls need [a limit on all guests
together](limits.md#guests-need-a-total) before they're answered: it's the only limit a visitor can't reset.

## When a call is refused

A refused call throws, whether a built-in limit refused it or one of your [onCall handlers](custom-limits.md) did.
Its message is written for the person who made the request ("limit reached: each guest, $0.10 a day (used $0.10); it
resets at 00:00 UTC", or your handler's own words), so the simplest handling is to show it:

```ts
// pb_pages/ask/+api.ts
export async function post(ctx: pages.Context) {
  const { question } = ctx.req.body()
  try {
    const r: LlmTextAnswer = await $llm.for(ctx).text.completeAsync({ model: "vultr/glm-5.3", prompt: question })
    return ctx.res.json({ answer: r.text })
  } catch (err: any) {
    return ctx.res.json(429, { error: String(err.message || err) })
  }
}
```

A streamed answer throws the same way, before any text arrives. With server-sent events, send the message as an event
the page shows in place of the answer, as [Docs Chat](/docs/examples/docs-chat) does:

```ts
// pb_pages/chat/+api.ts
export async function post(ctx: pages.Context) {
  const { question } = ctx.req.body()
  const stream = ctx.res.sse()
  try {
    const r = await $llm.for(ctx).text.streamAsync({ model: "vultr/glm-5.3", prompt: question },
      (text) => stream.send("text", JSON.stringify(text)))
    stream.send("done", JSON.stringify({ usage: r.usage }))
  } catch (err: any) {
    stream.send("error", JSON.stringify(String(err.message || err)))
  }
  stream.close()
}
```

To tell a refused call apart from other failures (a wrong key, a provider that's down), check `err.code === "limit"`.
Over HTTP, refused calls answer `429` with `{"code": "limit", ...}`.

## Showing what's left

`$llm.for(ctx).ledger.remaining()` lists the limits that count this visitor, with what's used and left, so a page can
say "4 questions left today" before anyone hits a refusal ([Spend and the ledger](spend.md#whats-left)):

```ets
<%% const left = $llm.for(ctx).ledger.remaining()[0] %>
<%% if (left) { %><p><%%= left.left %> of <%%= left.amount %> left today</p><%% } %>
```

## In Go

A custom build makes calls through the `llm.*` contracts. The subject travels in the call's context, set with the
helpers in the shared contract package:

```go
import (
	llmc "pocketone.dev/ext/contracts/llm"
	textv1 "pocketone.dev/ext/contracts/llm/text/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/llm"
)

func answer(app core.App, e *core.RequestEvent, question string) (string, error) {
	// whom the request is for: its auth record, else its guest cookie (set on e's response when missing), else its IP
	ctx := llmc.WithSubject(e.Request.Context(), llm.SubjectOf(app, e))
	r, err := textv1.Contract.Must(app).Complete(ctx, extkit.Internal, textv1.CompleteInput{
		Model: "vultr/glm-5.3", Prompt: question,
	})
	return r.Text, err
}
```

- `llmc.WithSubject(ctx, s)` and `llmc.SubjectOf(ctx)` set and read the subject; `llmc.UserSubject(collection, id)`
  builds a user's. A call whose context has no subject counts toward the whole app only.
- `llm.SubjectOf(app, e)` is what `$llm.for(e)` uses: the request's user, guest or IP.
- Pass the context on and the subject goes with it: LLM Chat's calls to `llm.text/v1` count for the subject of the
  send that made them.
- A refused call returns an `*extkit.Error` with `Code` `"limit"` and `Status` `429` (`llmc.ErrLimit`'s).
