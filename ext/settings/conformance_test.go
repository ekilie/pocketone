package settings_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/settings"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return settings.New() })
}
