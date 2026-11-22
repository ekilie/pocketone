// Package transcribev1 is the llm.transcribe/v1 contract: a model writes down what's said in audio.
//
// A wire contract at /api/_c/llm.transcribe/v1. In pb_hooks it's $llm.transcribe.
package transcribev1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	"pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

// Transcribe turns audio into text.
//
//contract:id llm.transcribe/v1
//contract:kind wire
//contract:scope llm
//contract:js hooks
type Transcribe interface {
	// Transcribe writes down what's said in the audio.
	//
	//op: POST transcribe
	Transcribe(ctx context.Context, a extkit.Auth, in TranscribeInput) (Result, error)
	// Models lists the enabled transcription models across providers, and the default.
	//
	//op: GET models scope=llm:read
	Models(ctx context.Context, a extkit.Auth, in llm.ModelsInput) (llm.Models, error)
}

// TranscribeInput is the audio, with hints.
type TranscribeInput struct {
	Model string   `json:"model,omitempty"`
	Audio llm.File `json:"audio"` // its Name's extension (story.mp3) or Type tells the provider the format
	// Language is the spoken language as an ISO code ("en"), when known; empty lets the model tell.
	Language string `json:"language,omitempty"`
	// Prompt is text that helps the model (names, words it might not know), for models that take one.
	Prompt  string      `json:"prompt,omitempty"`
	Options llm.Options `json:"options,omitempty"`
}

// Result is what was said.
type Result struct {
	Text     string    `json:"text"`
	Language string    `json:"language,omitempty"`
	Model    string    `json:"model"`
	Usage    llm.Usage `json:"usage"`
}
