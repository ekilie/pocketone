package s3fallback_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/core/multisite-s3-fallback"
	"github.com/pockethost/pocketone/ext/core/pluginkit"
	"github.com/pockethost/pocketone/ext/core/pluginkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() pluginkit.Plugin { return s3fallback.New(s3fallback.Config{}) })
}
