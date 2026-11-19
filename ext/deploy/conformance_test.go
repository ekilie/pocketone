package deploy_test

import (
	"testing"

	"pocketone.dev/ext/deploy"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return deploy.NewExtension(deploy.Config{}) })
}
