package autotls_test

import (
	"testing"

	"pocketone.dev/ext/autotls"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return autotls.NewApp(autotls.Config{}) })
}
