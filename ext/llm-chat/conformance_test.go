package llmchat_test

import (
	"testing"

	"pocketone.dev/ext/llm"
	llmchat "pocketone.dev/ext/llm-chat"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return llmchat.New() },
		conformance.Options{Deps: []extkit.Extension{llm.New()}})
}
