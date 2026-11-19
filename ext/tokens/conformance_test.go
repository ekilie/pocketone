package tokens_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
	"github.com/pockethost/pocketone/ext/tokens"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return tokens.New(tokens.Config{}) }, conformance.Options{
		AllowCollections: map[string]string{
			"_tokens": "API tokens, named before the _<name>_<table> rule; renamed to _tokens_tokens by a migration later",
		},
	})
}
