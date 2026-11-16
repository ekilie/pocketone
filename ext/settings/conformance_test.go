package settings_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
	"github.com/pockethost/pocketone/ext/settings"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return settings.New() })
}
