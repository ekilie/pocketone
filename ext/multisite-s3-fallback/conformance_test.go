package s3fallback_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
	"github.com/pockethost/pocketone/ext/multisite-s3-fallback"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return s3fallback.New(s3fallback.Config{}) })
}
