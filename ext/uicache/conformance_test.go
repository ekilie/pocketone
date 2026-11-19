package uicache_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
	"github.com/pockethost/pocketone/ext/uicache"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return uicache.New() })
}
