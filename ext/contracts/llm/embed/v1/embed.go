// Package embedv1 is the llm.embed/v1 contract: a model turns texts into vectors, for search by meaning.
//
// A wire contract at /api/_c/llm.embed/v1. In pb_hooks it's $llm.embed.
package embedv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	"pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

// Embed turns texts into vectors.
//
//contract:id llm.embed/v1
//contract:kind wire
//contract:scope llm
//contract:js hooks
type Embed interface {
	// Embed returns one vector per input text, in order.
	//
	//op: POST embed
	Embed(ctx context.Context, a extkit.Auth, in EmbedInput) (Result, error)
	// Models lists the enabled embedding models across providers, and the default.
	//
	//op: GET models scope=llm:read
	Models(ctx context.Context, a extkit.Auth, in llm.ModelsInput) (llm.Models, error)
}

// EmbedInput is the texts.
type EmbedInput struct {
	Model string   `json:"model,omitempty"`
	Input []string `json:"input"`
	// Dimensions shortens the vectors, for models that allow it.
	Dimensions int         `json:"dimensions,omitempty"`
	Options    llm.Options `json:"options,omitempty"`
}

// Result is the vectors, one per input.
type Result struct {
	Vectors [][]float64 `json:"vectors"`
	Model   string      `json:"model"`
	Usage   llm.Usage   `json:"usage"`
}
