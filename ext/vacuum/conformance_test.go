package vacuum_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/vacuum"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return vacuum.New() })
}
