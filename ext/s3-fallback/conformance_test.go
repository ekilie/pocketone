package s3fallback_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/s3-fallback"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return s3fallback.New(s3fallback.Config{}) })
}
