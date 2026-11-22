// Package speechv1 is the llm.speech/v1 contract: a model reads text aloud.
//
// A wire contract at /api/_c/llm.speech/v1. In pb_hooks it's $llm.speech.
package speechv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	"pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

// Speech turns text into audio.
//
//contract:id llm.speech/v1
//contract:kind wire
//contract:scope llm
//contract:js hooks
type Speech interface {
	// Speak reads the text aloud and returns the audio.
	//
	//op: POST speak
	Speak(ctx context.Context, a extkit.Auth, in SpeakInput) (Result, error)
	// Voices lists a model's voices, when the provider says.
	//
	//op: POST voices scope=llm:read
	Voices(ctx context.Context, a extkit.Auth, in VoicesInput) (VoiceList, error)
	// Models lists the speech models of every provider in Settings → LLM.
	//
	//op: GET models scope=llm:read
	Models(ctx context.Context, a extkit.Auth, in llm.ModelsInput) (llm.Models, error)
}

// SpeakInput is what to say and how.
type SpeakInput struct {
	Model string `json:"model"` // "provider/model", required
	Text  string `json:"text"`
	Voice string `json:"voice,omitempty"`
	// Format is the audio format: mp3 (default), wav, opus, flac, pcm, as the provider allows.
	Format string  `json:"format,omitempty"`
	Speed  float64 `json:"speed,omitempty"`
	// Instructions describe the delivery ("slow and calm, a bedtime voice"), for models that take them.
	Instructions string      `json:"instructions,omitempty"`
	Options      llm.Options `json:"options,omitempty"`
}

// Result is the audio.
type Result struct {
	File  llm.File  `json:"file"`
	Model string    `json:"model"`
	Usage llm.Usage `json:"usage"`
}

// VoicesInput names the model whose voices to list.
type VoicesInput struct {
	Model string `json:"model"` // "provider/model", required
}

// VoiceList is a model's voices.
type VoiceList struct {
	Voices []Voice `json:"voices"`
}

// Voice is one voice.
type Voice struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Language string `json:"language,omitempty"`
	Gender   string `json:"gender,omitempty"`
}
