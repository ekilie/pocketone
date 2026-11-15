package tokens_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
	"github.com/pockethost/pocketone/ext/tokens"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return tokens.New(tokens.Config{}) })
}
