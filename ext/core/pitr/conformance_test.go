package pitr_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/core/pitr"
	"github.com/pockethost/pocketone/ext/core/pluginkit"
	"github.com/pockethost/pocketone/ext/core/pluginkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() pluginkit.Plugin { return pitr.New(pitr.Config{}) })
}
