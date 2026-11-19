package s3redirect_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/s3redirect"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return s3redirect.New(s3redirect.Config{}) })
}
