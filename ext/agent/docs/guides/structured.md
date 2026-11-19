---
title: Structured answers
description: "Give $agent a JSON schema and get back data you can store: the built-in model can only write JSON that matches it."
---

# Structured answers

Most of what your code wants from a model is data, not prose: a label, a few fields pulled out of an email, a list of
tags. Pass a JSON schema as `schema`, and the answer's parsed value is `r.data`:

```ts
const r: AgentAnswer = $agent.complete({
  system: "Extract the order from the email.",
  prompt: email,
  schema: {
    type: "object",
    properties: {
      name: { type: "string" },
      city: { type: "string" },
      items: {
        type: "array",
        maxItems: 10,
        items: {
          type: "object",
          properties: { what: { type: "string" }, count: { type: "integer" } },
          required: ["what", "count"],
        },
      },
    },
    required: ["name", "city", "items"],
  },
  maxTokens: 200,
})
// "Hi, this is Dana. Please send 3 blue mugs and 2 red plates to our Austin office."
// r.data: { name: "Dana", city: "Austin", items: [{ what: "blue mugs", count: 3 }, { what: "red plates", count: 2 }] }
```

```go
r, err := agentv1.Contract.Must(app).Complete(ctx, extkit.Internal, agentv1.CompleteInput{
	System: "Extract the order from the email.",
	Prompt: email,
	Schema: json.RawMessage(`{
	  "type": "object",
	  "properties": {
	    "name": {"type": "string"},
	    "city": {"type": "string"},
	    "items": {"type": "array", "maxItems": 10, "items": {"type": "object",
	      "properties": {"what": {"type": "string"}, "count": {"type": "integer"}}, "required": ["what", "count"]}}
	  },
	  "required": ["name", "city", "items"]
	}`),
	MaxTokens: 200,
})
var order struct {
	Name, City string
	Items      []struct {
		What  string
		Count int
	}
}
err = json.Unmarshal(r.Data, &order) // {Dana Austin [{blue mugs 3} {red plates 2}]}
```

## How the schema is enforced

With a built-in model the schema isn't a suggestion the model might ignore: as it writes, every token that would
break the schema is ruled out, so the answer is always JSON of that shape. Even `qwen3-0.6b` fills in a schema
correctly. The engine enforces `type`, `enum`, `const`, `properties`, `required`, `additionalProperties`, `items`,
`minItems`, `maxItems`, `anyOf` and `oneOf`; other keywords (formats, patterns, lengths) are allowed but not checked,
so check those yourself.

With your own API, the schema goes to the provider as `response_format`. Most enforce it; if one doesn't and the
answer isn't JSON, `$agent.complete` throws `bad_answer`.

## Tips

- An `enum` is the most reliable way to get one of a few labels.
- Keep `maxTokens` a little above what the JSON needs; an answer cut off by `maxTokens` isn't valid JSON.
- The schema is also added to the system prompt, so the model knows what each field is for: name fields plainly.
