package pitr_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/pitr"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return pitr.New(pitr.Config{}) })
}
