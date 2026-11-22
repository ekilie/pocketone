// Package textv1 is the llm.text/v1 contract: a language model answers a prompt or a conversation. The model is any
// enabled text model of any provider the app's admins added in Settings → LLM ("vultr/glm-5.2"), or the text default.
//
// A wire contract at /api/_c/llm.text/v1. In pb_hooks it's $llm.text.
package textv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"
	"encoding/json"

	"pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

// Text answers prompts with a language model.
//
//contract:id llm.text/v1
//contract:kind wire
//contract:scope llm
//contract:js hooks
type Text interface {
	// Complete answers a prompt or a conversation.
	//
	//op: POST complete
	Complete(ctx context.Context, a extkit.Auth, in CompleteInput) (CompleteResult, error)
	// Stream answers like Complete, sending the answer as it's written: a chunk with text (or reasoning) for each
	// piece, then one with done and the whole result.
	//
	//op: POST stream
	Stream(ctx context.Context, a extkit.Auth, in CompleteInput) (extkit.Stream, error)
	// Count says about how many tokens a text or a conversation is, for budgeting what to send.
	//
	//op: POST count scope=llm:read
	Count(ctx context.Context, a extkit.Auth, in CountInput) (CountResult, error)
	// Models lists the enabled text models across providers, and the default.
	//
	//op: GET models scope=llm:read
	Models(ctx context.Context, a extkit.Auth, in llm.ModelsInput) (llm.Models, error)
}

// CompleteInput is a prompt: Prompt, or Messages for a conversation (Prompt is then added as the last user message).
type CompleteInput struct {
	// Model is "provider/model"; empty uses the text default.
	Model    string    `json:"model,omitempty"`
	System   string    `json:"system,omitempty"`
	Prompt   string    `json:"prompt,omitempty"`
	Messages []Message `json:"messages,omitempty"`
	// Schema, when set, is a JSON schema the answer must match; the result's Data is then the parsed answer.
	Schema json.RawMessage `json:"schema,omitempty"`
	// MaxTokens caps the answer's length, thinking included (0: the provider's default).
	MaxTokens   int      `json:"maxTokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"` // nil: the provider's default
	Stop        []string `json:"stop,omitempty"`
	// Reasoning is how much a reasoning model thinks first: "" (its default), "off", "low" or "high". A model that
	// doesn't reason ignores it.
	Reasoning string      `json:"reasoning,omitempty"`
	Options   llm.Options `json:"options,omitempty"`
}

// Message is one turn of a conversation.
type Message struct {
	Role    string `json:"role"` // system, user or assistant
	Content string `json:"content"`
}

// CompleteResult is the model's answer.
type CompleteResult struct {
	Text string          `json:"text"`
	Data json.RawMessage `json:"data,omitempty"` // the parsed answer, when a Schema was given
	// Reasoning is the model's thinking, when the provider returns it separately.
	Reasoning  string    `json:"reasoning,omitempty"`
	StopReason string    `json:"stopReason,omitempty"` // "stop", "length" or "cancelled"
	Model      string    `json:"model"`                // the provider/model that answered
	Usage      llm.Usage `json:"usage"`
}

// Chunk is one entry of a streamed answer: a piece of text (or of reasoning) as it's written, or, last, done with the
// whole result.
type Chunk struct {
	Text      string          `json:"text,omitempty"`
	Reasoning string          `json:"reasoning,omitempty"`
	Done      bool            `json:"done,omitempty"`
	Result    *CompleteResult `json:"result,omitempty"`
}

// CountInput is a text, or a conversation as a call would send it.
type CountInput struct {
	Model    string    `json:"model,omitempty"`
	Text     string    `json:"text,omitempty"`
	System   string    `json:"system,omitempty"`
	Messages []Message `json:"messages,omitempty"`
}

// CountResult is a token count. Exact is false when it's an estimate (about four characters a token).
type CountResult struct {
	Tokens int  `json:"tokens"`
	Exact  bool `json:"exact"`
}
