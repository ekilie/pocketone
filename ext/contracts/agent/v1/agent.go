// Package agentv1 is the agent/v1 contract: an app asks a language model for an answer. Who answers is the
// provider's business: the app's own OpenAI-compatible API (an endpoint and key its admins set), a model PocketOne
// runs itself, or a model a multisite host runs for all of its instances. App code calls the same thing in every case
// ($agent.complete in pb_hooks).
//
// A wire contract at /api/_c/agent/v1. A multisite host may provide it for its instances.
package agentv1

//go:generate go run github.com/pockethost/pocketone/ext/contracts/cmd/contractgen

import (
	"context"
	"encoding/json"

	"github.com/pockethost/pocketone/ext/extkit"
)

// Agent answers prompts with a language model.
//
//contract:id agent/v1
//contract:kind wire
//contract:scope agent
//contract:host
//contract:js hooks
type Agent interface {
	// Complete answers a prompt or a conversation with the app's language model.
	//
	//op: POST complete
	Complete(ctx context.Context, a extkit.Auth, in CompleteInput) (CompleteResult, error)
	// Stream answers like Complete, sending the answer as it's written: a chunk with text for each piece, then one
	// with done and the whole result.
	//
	//op: POST stream
	Stream(ctx context.Context, a extkit.Auth, in CompleteInput) (extkit.Stream, error)
	// Status says which model answers the app's prompts and whether it's ready.
	//
	//op: GET / scope=agent:read
	Status(ctx context.Context, a extkit.Auth) (Status, error)
}

// CompleteInput is a prompt: Prompt, or Messages for a conversation (Prompt is then added as the last user message).
type CompleteInput struct {
	System   string    `json:"system,omitempty"`
	Prompt   string    `json:"prompt,omitempty"`
	Messages []Message `json:"messages,omitempty"`
	// Schema, when set, is a JSON schema the answer must match; the result's Data is then the parsed answer.
	Schema json.RawMessage `json:"schema,omitempty"`
	// MaxTokens caps the answer's length (0: the provider's default). A host may cap it lower.
	MaxTokens   int      `json:"maxTokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"` // nil: the provider's default
	Stop        []string `json:"stop,omitempty"`
	// Model is a hint: a provider with one model ignores it.
	Model string `json:"model,omitempty"`
}

// Message is one turn of a conversation.
type Message struct {
	Role    string `json:"role"` // system, user or assistant
	Content string `json:"content"`
}

// CompleteResult is the model's answer.
type CompleteResult struct {
	Text  string          `json:"text"`
	Data  json.RawMessage `json:"data,omitempty"` // the parsed answer, when a Schema was given
	Model string          `json:"model"`
	// Backend is who answered: "app" (the app's own API), "runner" (a model PocketOne runs) or "host" (its host's model).
	Backend    string `json:"backend"`
	StopReason string `json:"stopReason,omitempty"` // "stop", "length" or "cancelled"
	Usage      Usage  `json:"usage"`
}

// Chunk is one entry of a streamed answer: a piece of text as it's written, or, last, done with the whole result.
type Chunk struct {
	Text   string          `json:"text,omitempty"`
	Done   bool            `json:"done,omitempty"`
	Result *CompleteResult `json:"result,omitempty"`
}

// Usage counts tokens.
type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// Status is the app's model.
type Status struct {
	Backend string `json:"backend"` // "app", "runner", "host" or "" (none configured)
	Model   string `json:"model,omitempty"`
	// Ready is false while the model downloads or loads; Progress is then 0..1 and Message says what's happening.
	Ready    bool    `json:"ready"`
	Progress float64 `json:"progress,omitempty"`
	Message  string  `json:"message,omitempty"`
}

// ErrNotConfigured: nothing answers the app's prompts yet.
var ErrNotConfigured = &extkit.Error{Code: "not_configured", Status: 503,
	Message: "no LLM configured: choose one in Settings → LLM, or name a model in pocketone.yaml's agent section"}

// ErrNotReady: the model is still downloading or loading.
var ErrNotReady = &extkit.Error{Code: "not_ready", Status: 503, Message: "the model is still loading; try again shortly"}
