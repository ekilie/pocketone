package autotls_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/autotls"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return autotls.NewApp(autotls.Config{}) })
}
