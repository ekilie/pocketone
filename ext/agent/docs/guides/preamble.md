---
title: Writing a preamble
description: "The system prompt that sets who the assistant is and how it answers: what to put in it, in what order, and how to keep it short enough for a small model."
---

# Writing a preamble

The preamble is the `system` text you send with every call: who the assistant is, what it may use, and how it
answers. It's the biggest lever you have on the answers, and the one people most often get wrong by writing too much.

```js
const r = $agent.complete({
  system: preamble, // the same text on every call
  messages: [...recentTurns, { role: "user", content: question + "\n\nDocumentation excerpts:\n\n" + excerpts }],
  maxTokens: 350,
})
```

```ts
const r: AgentAnswer = $agent.complete({
  system: preamble, // the same text on every call
  messages: [...recentTurns, { role: "user", content: question + "\n\nDocumentation excerpts:\n\n" + excerpts }],
  maxTokens: 350,
})
```

```go
r, err := agent.Complete(ctx, extkit.Internal, agentv1.CompleteInput{
	System: preamble, // the same text on every call
	Messages: append(recentTurns,
		agentv1.Message{Role: "user", Content: question + "\n\nDocumentation excerpts:\n\n" + excerpts}),
	MaxTokens: 350,
})
```

## What goes in it

Four short parts, in this order:

1. **Who it is and for whom.** One or two sentences: "You are the help assistant for Acme Notes, a note-taking app.
   You answer questions from people using the app."
2. **What it may use.** Say where answers come from: "Answer from the excerpts below." Without that line, a model
   happily fills gaps from what it thinks it knows.
3. **The rules.** A short list, each a thing to do: "If the excerpts don't answer the question, say you don't know and
   suggest where to look." "Give settings and commands exactly as written, in backticks." "Never promise refunds;
   point people to support@acme.example."
4. **The shape of an answer.** "Answer in a few sentences or a short list." If you need data rather than prose, use a
   [schema](structured.md) instead of describing the format in words.

The material it should use (the [excerpts](context.md) that match the question) goes with the question, not in the
preamble: see [Keep it the same](#keep-it-the-same).

```markdown
You are the help assistant for Acme Notes, a note-taking app. You answer questions from people using it.

Answer from the documentation excerpts below.

- If the excerpts don't answer the question, say you don't know and suggest where in the docs to look.
- Give settings, menu names and commands exactly as the excerpts write them, in `backticks`.
- Never promise refunds or account changes; point people to support@acme.example.

Answer in a few sentences or a short list.
```

## Keep it short

A built-in model reads the whole preamble on every call, and a small model follows a few clear rules much better than
many. Aim for under 150 words.

- **One rule per line**, phrased as what to do, not what to avoid ("answer in English" works better than "don't
  answer in other languages").
- **Cut anything the model does anyway.** "Be helpful and polite" changes nothing.
- **Don't paste your docs into the preamble.** Pick the passages that match each question and add them after it
  ([Your own knowledge](context.md)).
- **One example beats a paragraph of description.** If answers keep coming out in the wrong shape, add a single
  question and the answer you want, as a user and an assistant turn before the real conversation.

## Keep it the same

Write the preamble once, as a file (`pb_pages/_private/preamble.md`, read with `$os.readFile`), and send it unchanged
on every call, first. Then the conversation, then the new question with its excerpts.

That order matters for speed. The built-in model, like OpenAI, Anthropic and Google, keeps the work it did on the start
of a prompt and reuses it when the next prompt starts with exactly the same text, so a long, unchanging preamble is
read once, not on every call. Anything that varies at the top (today's date, the user's name, the excerpts) breaks
that: put it in the user's turn instead. See [Prompt caching](../prompt-caching.md).

## Test it

Ask it the questions it should refuse or hand off, not only the easy ones:

- something the excerpts don't cover (it should say it doesn't know, not guess);
- something against a rule (a refund request);
- a follow-up that only makes sense with the turn before it.

Change one line at a time and ask the same questions again. Turn `temperature` down (0.2 to 0.3) while you test, so
differences come from your change, not chance.
