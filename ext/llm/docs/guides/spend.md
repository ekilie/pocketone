---
title: Spend and the ledger
description: "Every model call is a row in _llm_calls: what it was, whom it was for and what it cost. Read spend per user or model, and sell credits from a hook."
---

# Spend and the ledger

Every call that reaches a provider, and every call a [limit](limits.md) refuses, leaves a row in the `_llm_calls`
collection: what kind of call it was, which model answered, whom it was for, what it used and what it cost. Limits count
from it, Settings → LLM shows today's and this month's spend from it, and your app can read it like any other
collection.

## What a row holds

| Field | |
|---|---|
| `kind` | `text`, `image`, `speech`, `transcribe`, `align`, `music`, `video` or `embed`. |
| `model` | The model, as `provider/model`. |
| `subject` | Whom the call was for: `user:<collection>/<id>`, `guest:<id>`, `guest:ip:<address>`, or empty for calls made with plain `$llm` ([Whom a call is for](whom.md)). |
| `via` | How the call arrived: `go` (page code and hooks), `http`, `mcp`. |
| `input`, `cached`, `output` | Input tokens, the input tokens billed at the cheaper cached rate, and output tokens. |
| `seconds` | Seconds of audio or video, for the kinds billed by length. |
| `cost` | What the call cost, in millionths of a US dollar (`57000` is $0.057). |
| `estimated` | True when the cost was worked out from the model's listed prices because the provider didn't report one. |
| `refused` | Why a limit refused the call (its message); empty for a call that reached the provider. |
| `job` | A music or video job's id. Its cost is filled in once the job is done ([Limits](limits.md#dollars-need-prices)). |
| `created` | When the call was made. |

It's a system collection with no API rules: superusers see it in the dashboard (Collections → `_llm_calls`), and your
app's own code reads it with `$app`. Visitors can't list it. Show them what's theirs through your own routes.

Rows are kept for 13 months. A daily job deletes older ones, since limits only look back a month.

## Reading spend

What a signed-in user has spent today, for a page that shows it:

```ts
// pb_pages/usage/+page.ts
export function load(ctx: pages.Context) {
  if (!ctx.auth) return { spent: 0 }
  const since = new Date().toISOString().slice(0, 10) + " 00:00:00.000Z" // today, UTC
  const row = new DynamicModel({ cost: 0 })
  $app.db()
    .newQuery("SELECT COALESCE(SUM(cost), 0) AS cost FROM _llm_calls WHERE subject = {:s} AND created >= {:since} AND refused = ''")
    .bind({ s: `user:${ctx.auth.collection().name}/${ctx.auth.id}`, since })
    .one(row)
  return { spent: row.cost / 1e6 } // dollars
}
```

This month's spend by model, for an admin page:

```sql
SELECT model, COUNT(*) AS calls, SUM(cost) / 1e6 AS dollars
FROM _llm_calls
WHERE created >= strftime('%Y-%m-01 00:00:00.000Z', 'now') AND refused = ''
GROUP BY model ORDER BY dollars DESC
```

And how often limits refused calls today, by limit:

```sql
SELECT refused, COUNT(*) AS times
FROM _llm_calls
WHERE created >= strftime('%Y-%m-%d 00:00:00.000Z', 'now') AND refused != ''
GROUP BY refused ORDER BY times DESC
```

Each of these runs in the dashboard's SQL console too.

## Selling credits

Limits cap spending per period. If users buy credits, keep the balance on their own record and use the ledger to
spend it: every row is created like any record, so a hook sees each call as it's recorded.

```js
// pb_hooks/credits.pb.js: users have a "credits" number field, in millionths of a dollar
onRecordAfterCreateSuccess((e) => {
  const m = e.record.get("subject").match(/^user:users\/(.+)$/)
  if (m && e.record.get("cost") > 0) {
    e.app.db()
      .newQuery("UPDATE users SET credits = credits - {:cost} WHERE id = {:id}")
      .bind({ cost: e.record.get("cost"), id: m[1] })
      .execute()
  }
  e.next()
}, "_llm_calls")
```

Then check the balance before calling the model, and refuse in your own words:

```ts
// pb_pages/ask/+api.ts
export async function post(ctx: pages.Context) {
  if (!ctx.auth) throw ctx.httpError(401, "sign in first")
  if (ctx.auth.getInt("credits") <= 0) return ctx.res.json(402, { error: "You're out of credits." })
  const r: LlmTextAnswer = await $llm.for(ctx).text.completeAsync({ model: "vultr/glm-5.3", prompt: ctx.req.body().question })
  return ctx.res.json({ answer: r.text })
}
```

As with limits, the check comes before the call and the cost after it, so a user's last call can take them a little
below zero. Keep a whole-app limit as well, so a mistake in your own code can't run up the bill.
