package tokens_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/core/pluginkit"
	"github.com/pockethost/pocketone/ext/core/pluginkit/conformance"
	"github.com/pockethost/pocketone/ext/core/tokens"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() pluginkit.Plugin { return tokens.New(tokens.Config{}) })
}
