package s3redirect_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
	"github.com/pockethost/pocketone/ext/s3redirect"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return s3redirect.New(s3redirect.Config{}) })
}
