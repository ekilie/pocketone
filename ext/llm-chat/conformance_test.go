package llmchat_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/llm"
	llmchat "pocketone.dev/ext/llm-chat"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return llmchat.New() },
		conformance.Options{Deps: []extkit.Extension{llm.New()}})
}
