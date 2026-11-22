// Package alignv1 is the llm.align/v1 contract: forced alignment. Given a recording and the exact words spoken in it,
// a model says when each line and each word is said. Nothing is transcribed or guessed: every word that comes back is
// one that was sent, in the order sent. For captions timed to a narration, karaoke-style highlighting, or finding a
// phrase in a recording.
//
// A wire contract at /api/_c/llm.align/v1. In pb_hooks it's $llm.align.
package alignv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	"pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

// Align times the words of a recording.
//
//contract:id llm.align/v1
//contract:kind wire
//contract:scope llm
//contract:js hooks
type Align interface {
	// Align says when each line and word of the text is said in the audio.
	//
	//op: POST align
	Align(ctx context.Context, a extkit.Auth, in AlignInput) (Result, error)
	// Models lists the alignment models of every provider in Settings → LLM.
	//
	//op: GET models scope=llm:read
	Models(ctx context.Context, a extkit.Auth, in llm.ModelsInput) (llm.Models, error)
}

// AlignInput is the recording and its words.
type AlignInput struct {
	Model string   `json:"model"` // "provider/model", required
	Audio llm.File `json:"audio"` // its Name's extension (story.mp3) or Type tells the provider the format
	// Text is the words in the audio, one line per line. A line in square brackets ([Chorus]) labels the lines under it
	// and isn't timed.
	Text string `json:"text"`
	// Mode is "speech" for a voice recording, or "lyrics" (the provider's default) to separate a sung vocal from the
	// music first.
	Mode string `json:"mode,omitempty"`
	// Language is the text's language as an ISO 639-1 code ("en"); empty lets the model tell.
	Language string      `json:"language,omitempty"`
	Options  llm.Options `json:"options,omitempty"`
}

// Result is when each line and word is said.
type Result struct {
	// Duration is the audio's length in seconds.
	Duration float64 `json:"duration"`
	// Lines are the lines placed in the audio, in order (labels and blank lines aren't lines).
	Lines []Line `json:"lines"`
	// Words are every word of those lines, in order.
	Words []Word `json:"words"`
	// Unaligned are lines the model couldn't place, listed instead of left out.
	Unaligned []Unaligned `json:"unaligned,omitempty"`
	Model     string      `json:"model"`
	Usage     llm.Usage   `json:"usage"`
}

// Line is one line's place in the audio.
type Line struct {
	Index int     `json:"index"` // which line of Text, counting from 0, skipping labels and blank lines
	Text  string  `json:"text"`
	Start float64 `json:"start"` // seconds from the start of the audio to its first word
	End   float64 `json:"end"`   // seconds to the end of its last word
	// Score is the model's confidence (higher is better); Certain is its judgement that the line is placed right.
	Score   float64 `json:"score"`
	Certain bool    `json:"certain"`
	// Tag is the label of the line's section ("Chorus" for [Chorus]), when there is one.
	Tag string `json:"tag,omitempty"`
}

// Word is one word's place in the audio.
type Word struct {
	Line  int     `json:"line"` // the Index of its line
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Score float64 `json:"score"`
}

// Unaligned is a line the model couldn't place.
type Unaligned struct {
	Line   int    `json:"line"`
	Text   string `json:"text"`
	Reason string `json:"reason"`
}
