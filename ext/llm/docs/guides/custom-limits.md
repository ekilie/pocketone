---
title: Custom limits
description: "Your own limits in code: $llm.onCall runs before every model call and can change it or refuse it, $llm.onCallDone runs after with what it cost. Plan tiers, credits, a model per plan and team buckets."
---

# Custom limits

The [limits in Settings → LLM](limits.md) cover how much the app, each user and each guest may spend per day or month.
Anything that depends on your app, such as a user's plan, credits they bought, or the team they belong to, goes in your
own code, in two hooks every model call passes through:

- **`$llm.onCall`** runs before every call that reaches a provider, of every kind. It can change the call (its model,
  its input, whom it counts for) or refuse it.
- **`$llm.onCallDone`** runs after a call that reached a provider, with what it used and cost. It can't refuse; the
  call has happened.

The limits in Settings → LLM are themselves an `onCall` handler, built in. Yours run before it, so the built-in limits
count the model and the subject your code settled on.

```js
// pb_hooks/limits.pb.js
$llm.onCall((e) => {
  // e.kind ("text", "image", ...), e.op, e.model, e.input, e.subject, e.user ({ collection, id } or null), e.guest
  if (e.input.prompt?.length > 20000) throw e.refuse("That question is too long.")
  e.next()
})
```

## onCall

A handler gets one call on its way to a provider:

| | |
|---|---|
| `e.kind` | `text`, `image`, `speech`, `transcribe`, `align`, `music`, `video` or `embed`. |
| `e.op` | The operation: `complete`, `stream`, `generate`, `edit`, `speak`, `transcribe`, `align`, `create` (music and video) or `embed`. |
| `e.model` | The model, as `provider/model`. Set it to send the call to another model. |
| `e.input` | The call's input: `prompt`, `messages`, `maxTokens`, `size`, ... as the page passed it. Change any of it. |
| `e.subject` | Whom the call is for (see [Whom a call is for](whom.md)). Set it to count the call for someone else, or for a name of your own (`team:acme`). |
| `e.user` | The signed-in user the call is for, as `{ collection, id }`, or `null`. |
| `e.guest` | `true` when the call is for a guest. |
| `e.guestsCapped` | Set it to `true` when your handler takes care of what guests spend (see below). |

Then it does one of two things:

- **`e.next()`** lets the call go on, with whatever the handler changed.
- **`throw e.refuse("message")`** refuses it. The page's `$llm` call throws with code `limit` (HTTP 429) and your
  message, the same as a built-in limit's refusal, so one `catch` handles both, and the ledger records it as refused,
  with your message.

A handler that does neither refuses the call too, with a message saying a handler didn't call `e.next()`: forgetting
`next()` never lets a call through unchecked.

Handlers run in the order they're bound, all before the built-in limits. They run on the app's JS VM, but the model
call itself doesn't: the handler runs, then the model works off the VM, and the app keeps serving other requests. A
handler may be `async`, but every call waits for it, so keep it to quick reads.

## onCallDone

A handler gets a call that reached a provider, with what it used:

| | |
|---|---|
| `e.kind`, `e.op`, `e.model`, `e.subject`, `e.user`, `e.guest` | As in `onCall`, with the values the call was made with. |
| `e.usage` | What it used: `inputTokens`, `outputTokens`, `cachedTokens`, `seconds`, and `costMicroUSD`, what it cost in millionths of a dollar. |
| `e.job` | A music or video job's id. A job's `onCallDone` comes once it's done and its cost is known (when `createAsync` with `wait`, or a later `getAsync`, sees it finish). |

A refused call has no `onCallDone`. An error a handler throws is logged and doesn't affect the call. The ledger row is
already written when `onCallDone` runs.

## Plan tiers

Free users get 10 questions a day and the small model; paid users get the big one. The plan lives on the user's record;
the count comes from [the ledger](spend.md):

```js
// pb_hooks/plans.pb.js: users have a "plan" select field, "free" or "pro"
$llm.onCall((e) => {
  if (!e.user) return e.next() // guests and background work: the built-in limits handle them
  const user = $app.findRecordById(e.user.collection, e.user.id)
  if (user.get("plan") !== "pro") {
    e.model = "vultr/glm-5.3-mini"
    e.input.maxTokens = Math.min(e.input.maxTokens || 800, 800)
    if ($llm.ledger.spent({ subject: e.subject, window: "day" }).calls >= 10) {
      throw e.refuse("Free accounts get 10 questions a day. Upgrade for more, or come back tomorrow.")
    }
  }
  e.next()
})
```

## Credits

Users buy credits; every call spends them. Refuse before a call when the balance is empty, and take the cost off after:

```js
// pb_hooks/credits.pb.js: users have a "credits" number field, in millionths of a dollar
$llm.onCall((e) => {
  if (e.user && $app.findRecordById(e.user.collection, e.user.id).getInt("credits") <= 0) {
    throw e.refuse("You're out of credits.")
  }
  e.next()
})

$llm.onCallDone((e) => {
  if (e.user && e.usage.costMicroUSD > 0) {
    $app.db()
      .newQuery("UPDATE " + e.user.collection + " SET credits = credits - {:cost} WHERE id = {:id}")
      .bind({ cost: e.usage.costMicroUSD, id: e.user.id })
      .execute()
  }
  e.next()
})
```

The check comes before the call and the cost after it, so a user's last call can take their balance a little below
zero.

## Team buckets

Count every member's calls toward their team, so a team shares one allowance:

```js
// pb_hooks/teams.pb.js: users have a "team" relation
$llm.onCall((e) => {
  if (e.user) {
    const team = $app.findRecordById(e.user.collection, e.user.id).get("team")
    if (team) e.subject = "team:" + team
  }
  e.next()
})
```

The ledger then records the team as the subject, and `$llm.ledger.spent({ subject: "team:" + teamId, window: "month" })`
is what the team has used. The built-in limits on **each signed-in user** and **each guest** count users and guests,
not teams: a team's own allowance is a check in your handler, like the plan tier's above.

## Guests in your own code

Guest calls are refused until there's a limit on all guests together, because a guest can clear their cookies and come
back with a fresh allowance ([Limits](limits.md#guests-need-a-total)). If your handler takes care of guests itself,
say so with `e.guestsCapped = true`, and the built-in limits let guest calls through without that limit:

```js
$llm.onCall((e) => {
  if (e.guest) {
    // guests get the small model, short answers, and 200 calls a day between them
    e.model = "vultr/glm-5.3-mini"
    e.input.maxTokens = 300
    if ($llm.ledger.spent({ subject: "guests", window: "day" }).calls >= 200) {
      throw e.refuse("We've answered all the questions we can today. Come back tomorrow.")
    }
    e.subject = "guests" // count every guest in one bucket of our own
    e.guestsCapped = true
  }
  e.next()
})
```

Counting every guest under one subject of your own makes their shared total one `spent` call. The ledger then shows
`guests` instead of each guest's id; keep `e.subject` as it is if you'd rather see each one.

Only set it when your code really does cap what guests spend. Otherwise add the limit in Settings → LLM.

## In Go

The hooks are `llm.OnCall(app)` and `llm.OnCallDone(app)` in the shared contract package
(`pocketone.dev/ext/contracts/llm`), so a custom build, or another extension (a billing extension, say), binds them
the same way:

```go
import llmc "pocketone.dev/ext/contracts/llm"

llmc.OnCall(app).BindFunc(func(e *llmc.CallEvent) error {
	if p := llmc.ParseSubject(e.Subject); p.Kind == "user" && !isPro(app, p.Collection, p.ID) {
		e.Model = "vultr/glm-5.3-mini"
	}
	return e.Next() // or: return e.Refuse("...")
})

llmc.OnCallDone(app).BindFunc(func(e *llmc.CallDoneEvent) error {
	debit(app, e.Subject, e.Usage.CostMicroUSD)
	return e.Next()
})
```

`e.Input` is a pointer to the operation's input (`*textv1.CompleteInput` for text, `*imagev1.GenerateInput` for
pictures, ...). The built-in limits are bound with the id `llm.LimitsHandlerID` and priority `llm.LimitsPriority`
(1000): bind with a higher priority to run after them, or unbind them to replace them entirely.
