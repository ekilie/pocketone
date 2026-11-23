package llmsearch_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	llmsearch "pocketone.dev/ext/llm-search"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return llmsearch.New() }, conformance.Options{})
}
