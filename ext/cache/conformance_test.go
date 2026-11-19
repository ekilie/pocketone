package cache_test

import (
	"testing"

	"pocketone.dev/ext/cache"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return cache.New(cache.Config{}) })
}
