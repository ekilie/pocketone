package templates_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/templates"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return templates.New(templates.Config{}) })
}
