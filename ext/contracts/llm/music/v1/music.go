// Package musicv1 is the llm.music/v1 contract: a model makes music from a prompt (and lyrics). Music takes a while,
// so it's a job: create starts it (or waits for it, with wait), get follows it, content fetches its files.
//
// A wire contract at /api/_c/llm.music/v1. In pb_hooks it's $llm.music.
package musicv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	"pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

// Music makes music.
//
//contract:id llm.music/v1
//contract:kind wire
//contract:scope llm
//contract:js hooks
type Music interface {
	// Create starts making music. With Wait it returns once the job is done (or failed), its files included;
	// otherwise at once, to follow with Get.
	//
	//op: POST create
	Create(ctx context.Context, a extkit.Auth, in CreateInput) (llm.Job, error)
	// Get says how a job is going.
	//
	//op: POST get scope=llm:read
	Get(ctx context.Context, a extkit.Auth, in llm.JobRef) (llm.Job, error)
	// Content fetches one of a done job's files.
	//
	//op: POST content scope=llm:read
	Content(ctx context.Context, a extkit.Auth, in llm.ContentInput) (llm.File, error)
	// Models lists the music models of every provider in Settings → LLM.
	//
	//op: GET models scope=llm:read
	Models(ctx context.Context, a extkit.Auth, in llm.ModelsInput) (llm.Models, error)
}

// CreateInput is the music to make.
type CreateInput struct {
	Model  string `json:"model"`  // "provider/model", required
	Prompt string `json:"prompt"` // the style: "gentle lullaby, music box and soft strings"
	Lyrics string `json:"lyrics,omitempty"`
	// Seconds is how long (the provider's default when 0).
	Seconds float64 `json:"seconds,omitempty"`
	// Wait is how long to wait for the job, as a duration ("5m"); "" returns at once.
	Wait    string      `json:"wait,omitempty"`
	Options llm.Options `json:"options,omitempty"`
}
