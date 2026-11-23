---
title: Limits
description: "Cap what your app spends on models: limits in Settings → LLM for the whole app, each user, each guest and all guests together, by dollars, tokens or calls, per UTC day or month."
---

# Limits

Every model call spends money on your provider account. On a page anyone can open, every visitor spends it. Limits cap
that: you set them in **Settings → LLM**, and every call passes them before it reaches a provider. A call a used-up
limit refuses never reaches the provider and costs nothing.

The limits on this screen are PocketOne's built-in limits: one handler on the hook every call passes through
before it reaches a provider. Anything they can't express, such as a user's plan, credits they bought or the team
they're on, you write as your own handler on the same hook ([Custom limits](custom-limits.md)). Yours run first, and
the built-in limits count what they settled on.

This page covers the built-in limits. [Users and guests](users-and-guests.md) covers the one line page code adds so per-user and
per-guest limits work, and [Spend and the ledger](spend.md) covers the record every call leaves and how to read what's
left.

## Setting a limit

Open the dashboard, go to **Settings → LLM**, and scroll past the providers to **Limits**. Press **New limit**, fill
in the row, and press **Save changes**. Limits apply from the next call, with no restart.

![Settings → LLM: four limits (all guests together $1 a day, each guest $0.10 a day, the whole app $25 a month, and 20 video calls a month) under today's and this month's spend](/docs/extensions/llm/guides/limits-settings.png)

Each limit reads as a sentence: **who** may use **at most** so much of **what**, **per** day or month.

| Field | What it means |
|---|---|
| **Who** | Whose calls the limit counts. **The whole app**: every call. **Each signed-in user**: each user separately, so one user using theirs up leaves the others theirs. **Each guest**: each visitor who isn't signed in, separately. **All guests together**: every guest's calls in one shared bucket. |
| **Counts** | What it counts. **US dollars**: what the calls cost. **Output tokens**: what the model wrote. **Input tokens**: what was sent to it. **Calls**: how many calls reached a provider. |
| **At most** | The amount. Dollars may have cents (`0.10`); tokens and calls are whole numbers. `0` refuses every call the limit counts. |
| **Per** | **a day** or **a month**, in UTC: a day starts at 00:00 UTC, a month on the 1st at 00:00 UTC. |
| **Kind** | Count only one kind of call: `text`, `image`, `speech`, `transcribe`, `align`, `music`, `video` or `embed`. **Every kind** counts all of them. |
| **Model** | Count only calls to one model, named as `provider/model` (`vultr/glm-5.3`). Empty counts every model. |

A limit with a kind or a model counts only those calls toward its amount, and refuses only those calls. Add as many
limits as you like: a call has to pass every one that counts it.

Above the limits, the screen shows what the whole app has spent today and this month, and how many calls today a limit
refused. The numbers come from [the ledger](spend.md).

## When a limit is used up

Before each call, every limit that counts it is checked against what's been used so far in its window. If one is used
up, the call is refused before it reaches the provider. In page code the `$llm` call throws, over HTTP the contract
answers `429`, and either way the error's code is `limit` and its message names the limit and when it resets:

```
limit reached: each guest, $0.10 a day (used $0.10); it resets at 00:00 UTC
limit reached: the whole app, 20 calls a month of video (used 20 calls); it resets on Dec 1 (UTC)
```

The message is written to show to the person who made the call. [Users and guests](users-and-guests.md#when-a-call-is-refused)
shows how to catch it in a page.

A refused call still gets a row in the ledger, with the limit's message, so you can see how often visitors hit it.

## Guests need a total

A guest is told apart by a cookie (see [Users and guests](users-and-guests.md#guests)), and anyone can clear their cookies and
come back as a new guest with a fresh allowance. A limit on **each guest** keeps one visitor from using everything up
by accident, but it doesn't cap what guests spend. Only a limit on **all guests together** does.

So guest calls are refused until there's a limit on all guests together that counts them. Until you add one, a guest's
call fails with:

```
guest calls are off until the app caps what all guests together may spend: add a limit for all guests in Settings → LLM
```

and the Limits section shows a warning. Calls for signed-in users, and calls that aren't for anyone in particular,
aren't guest calls and aren't affected.

## Checked before, counted after

A call's cost and tokens are known only once the provider answers, so a limit is checked before each call and the call
is counted after it. Two things follow:

- **The last call a limit lets through can take it over**, by that call's own cost. With $0.10 left and a call that
  costs $0.15, the call goes through and the next one is refused. Keep your amounts a call or two under what you can
  afford.
- **Calls running at the same moment can all pass the check** before any of them is counted, so a burst of calls can
  take a limit further over than one call. Limits keep spending in the right range day to day; they aren't an exact
  meter.

## Dollars need prices

A call's cost is what the provider reports, or else what its listed prices say. A model with no price, such as a model
on your own machine (Ollama, llama-server) or an OpenAI-compatible server that lists no prices, costs $0 as far as the
ledger knows, so a dollar limit never fills up for it. Count **tokens** or **calls** for those models instead.

Music and video are jobs: a call starts the job, and the job's cost is known once it's done. A job counts as a call as
soon as it starts, and its cost is added to its ledger row once `createAsync` with `wait` sees it finish, or once
`getAsync` does. A job nobody checks again after starting keeps a cost of $0.

## Limits for common apps

**An internal tool**, where everyone is signed in and trusted: one ceiling.

- The whole app, US dollars, at most 20, per month.

**A public demo**, like an [ask your docs](/docs/starters/ask) page, where visitors aren't signed in:

- All guests together, US dollars, at most 1, per day.
- Each guest, US dollars, at most 0.10, per day.
- The whole app, US dollars, at most 20, per month.

**An app with accounts**, where users sign in to use the model:

- Each signed-in user, output tokens, at most 200000, per day.
- The whole app, US dollars, at most 50, per month.

**An expensive kind on its own**, next to whatever else you have:

- The whole app, calls, at most 20, per month, kind video.
- The whole app, US dollars, at most 5, per day, kind image.

Plan tiers, credits users buy, and anything else that depends on who the user is belong in your app's own code: see
[Custom limits](custom-limits.md).
