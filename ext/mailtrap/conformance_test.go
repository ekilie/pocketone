package mailtrap_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/mailtrap"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return mailtrap.New() })
}
