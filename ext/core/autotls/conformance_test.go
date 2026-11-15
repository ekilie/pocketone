package autotls_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/core/autotls"
	"github.com/pockethost/pocketone/ext/core/pluginkit"
	"github.com/pockethost/pocketone/ext/core/pluginkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() pluginkit.Plugin { return autotls.NewApp(autotls.Config{}) })
}
