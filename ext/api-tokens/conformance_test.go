package apitokens_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/api-tokens"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return apitokens.New(apitokens.Config{}) })
}
