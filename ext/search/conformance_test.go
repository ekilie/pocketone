package search_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/search"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return search.New() }, conformance.Options{})
}
