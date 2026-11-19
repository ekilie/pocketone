package remotes_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/remotes"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return remotes.New(remotes.Config{}) })
}
