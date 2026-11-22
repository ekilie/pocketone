---
title: Limits
description: "Cap what your app spends on models: limits in Settings → LLM for the whole app, each user, each guest and all guests together, checked before every call, with $llm.for(e) to say whom a call is for."
---

# Limits

Every call to a model spends money on your provider account, and a public page lets every visitor spend it. Limits
cap that. You set them in **Settings → LLM**, under the providers, and every call passes them before it reaches a
provider. Nothing to write in your pages beyond saying whom a call is for.

## A limit

Each limit is **who** × **what** × **at most** × **per**:

| | |
|---|---|
| Who | the whole app · each signed-in user · each guest · all guests together |
| What | US dollars · output tokens · input tokens · calls |
| Per | a day · a month (UTC: a day starts at 00:00 UTC) |

A limit can also count only one kind (`text`, `image`, `speech`...) or one model (`vultr/glm-5.3`). Some that go
together:

- The whole app, $5 a day, and $50 a month.
- Each signed-in user, 200,000 output tokens a day.
- All guests together, $1 a day, and each guest, $0.10 a day.
- The whole app, 20 `video` calls a month.

When a limit is used up, the call is refused before it reaches the provider: it throws with code `limit` (HTTP 429),
and the message names the limit and when it resets:

```
limit reached: each guest, $0.10 a day (used $0.10); it resets at 00:00 UTC
```

Limits are checked before a call and counted after it (a call's cost is known only once the provider answers), so the
last call a limit lets through can take it over by that call's own cost.

## Whom a call is for

Plain `$llm` calls count only toward limits on the whole app: the code making them could be serving anyone, or no one
(a cron). In code that serves a request, `$llm.for()` says whom it's for, and the per-user and per-guest limits apply:

```ts
// pb_pages/ask/+api.ts
export async function post(ctx: pages.Context) {
  const { question } = ctx.req.body()
  try {
    const r: LlmTextAnswer = await $llm.for(ctx).text.completeAsync({ model: "vultr/glm-5.3", prompt: question })
    return ctx.res.json({ answer: r.text })
  } catch (err: any) {
    // a used-up limit: tell the visitor to come back later
    return ctx.res.json(429, { error: String(err.message || err) })
  }
}
```

`$llm.for()` takes a page's `ctx` or a `pb_hooks` route's `e`, and gives the same API as `$llm` (chats and streams
included). The request is for:

- **its signed-in user**, when it carries an auth record;
- else **a guest**, told apart by a cookie (`pocketone_llm_guest`, signed so a visitor can't pick someone else's) that
  the first call sets. Until the browser sends the cookie back, the guest counts by IP address, so a client that drops
  cookies can't get a fresh allowance on every call.

## Guests need a total

A guest who clears their cookies becomes a new guest with a fresh allowance, so a limit on each guest doesn't cap what
guests spend. Only a limit on **all guests together** does. So guest calls are refused until you add one: an app that
serves visitors who aren't signed in needs an all-guests limit in Settings → LLM before it answers them. The screen
says so while there isn't one.

## What's been spent

Every call is recorded in the `_llm_calls` collection: when, the kind and model, whom it was for, the tokens and
seconds it used, what it cost, and, for a refused call, which limit refused it. Settings → LLM shows today's and this
month's spend from it.

The rows are ordinary records, so your app can use them: show a user what they've spent, or debit credits they bought
from an `onRecordAfterCreateSuccess` hook on `_llm_calls`. Anything the limits don't express (plans, tiers, credits)
belongs in your app's own code, before it calls `$llm`.
