// Package streamv1 is the agent.stream/v1 contract: agent/v1's answers, handed over a piece at a time as the model
// writes them, to code in the same process ($agent.stream in pb_hooks and pages, Go callers). A companion of agent/v1
// for in-process callers only: it has no HTTP API, because a callback can't cross one.
//
// Whoever provides agent/v1 provides it too, so both answer from the same backend (the app's own API, a model
// PocketOne runs, or the app's host). It isn't host-provided: under a multisite host whose instances run in its
// process (Pocker), the app's provider streams from the host's model directly.
package streamv1

//go:generate go run github.com/pockethost/pocketone/ext/contracts/cmd/contractgen

import (
	"context"

	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
)

// Streamer answers prompts a piece at a time.
//
//contract:id agent.stream/v1
type Streamer interface {
	// Stream answers in like agent/v1's Complete, calling in.OnText with each piece of the answer as it's written.
	// OnText returning false stops the answer there (the result says "cancelled"). The result is the whole answer.
	Stream(ctx context.Context, in Input) (Result, error)
}

// Result is agent/v1's answer.
type Result = agentv1.CompleteResult

// Input is agent/v1's input plus the callback that receives the answer as it's written.
type Input struct {
	agentv1.CompleteInput
	// OnText gets each piece of the answer in order; nil is allowed (Stream is then Complete).
	OnText func(text string) bool `json:"-"`
}
