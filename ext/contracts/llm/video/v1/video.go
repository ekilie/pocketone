// Package videov1 is the llm.video/v1 contract: a model makes a video from a prompt (and a first frame). Like music
// it's a job: create starts it (or waits for it, with wait), get follows it, content fetches its files.
//
// A wire contract at /api/_c/llm.video/v1. In pb_hooks it's $llm.video.
package videov1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	"pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

// Video makes videos.
//
//contract:id llm.video/v1
//contract:kind wire
//contract:scope llm
//contract:js hooks
type Video interface {
	// Create starts making a video. With Wait it returns once the job is done (or failed); otherwise at once, to
	// follow with Get. A done job lists how many files it made (Outputs); Content fetches each.
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
	// Models lists the video models of every provider in Settings → LLM.
	//
	//op: GET models scope=llm:read
	Models(ctx context.Context, a extkit.Auth, in llm.ModelsInput) (llm.Models, error)
}

// CreateInput is the video to make.
type CreateInput struct {
	Model  string `json:"model"` // "provider/model", required
	Prompt string `json:"prompt"`
	// Image is a first frame to start from, for models that take one.
	Image   *llm.File `json:"image,omitempty"`
	Seconds float64   `json:"seconds,omitempty"`
	Size    string    `json:"size,omitempty"` // "1280x720"
	// AspectRatio is "16:9", "9:16"... for models that take one instead of a size.
	AspectRatio string `json:"aspectRatio,omitempty"`
	// Wait is how long to wait for the job, as a duration ("10m"); "" returns at once.
	Wait    string      `json:"wait,omitempty"`
	Options llm.Options `json:"options,omitempty"`
}
