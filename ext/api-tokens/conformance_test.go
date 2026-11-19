package apitokens_test

import (
	"testing"

	"pocketone.dev/ext/api-tokens"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return apitokens.New(apitokens.Config{}) })
}
