// Package imagev1 is the llm.image/v1 contract: a model makes pictures from a prompt, or changes pictures it's given.
//
// A wire contract at /api/_c/llm.image/v1. In pb_hooks it's $llm.image.
package imagev1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	"pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

// Image makes pictures.
//
//contract:id llm.image/v1
//contract:kind wire
//contract:scope llm
//contract:js hooks
type Image interface {
	// Generate makes pictures from a prompt.
	//
	//op: POST generate
	Generate(ctx context.Context, a extkit.Auth, in GenerateInput) (Result, error)
	// Edit changes the pictures it's given as the prompt says.
	//
	//op: POST edit
	Edit(ctx context.Context, a extkit.Auth, in EditInput) (Result, error)
	// Models lists the image models of every provider in Settings → LLM.
	//
	//op: GET models scope=llm:read
	Models(ctx context.Context, a extkit.Auth, in llm.ModelsInput) (llm.Models, error)
}

// GenerateInput is what to draw.
type GenerateInput struct {
	Model  string `json:"model"` // "provider/model", required
	Prompt string `json:"prompt"`
	N      int    `json:"n,omitempty"`    // how many (default 1)
	Size   string `json:"size,omitempty"` // "1024x1024"
	// AspectRatio is "16:9", "1:1"... for models that take one instead of a size.
	AspectRatio string      `json:"aspectRatio,omitempty"`
	Options     llm.Options `json:"options,omitempty"`
}

// EditInput is what to change, and the pictures to change.
type EditInput struct {
	GenerateInput
	Images []llm.File `json:"images"`
}

// Result is the pictures made.
type Result struct {
	Files []llm.File `json:"files"`
	Model string     `json:"model"`
	Usage llm.Usage  `json:"usage"`
}
