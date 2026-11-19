package pages_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/pages"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return pages.New(pages.Config{}) })
}
