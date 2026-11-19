package desktop_test

import (
	"testing"

	"pocketone.dev/ext/desktop"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return desktop.New() })
}
