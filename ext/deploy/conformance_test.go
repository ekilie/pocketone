package deploy_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/deploy"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return deploy.NewExtension(deploy.Config{}) }, conformance.Options{
		AllowCollections: map[string]string{
			"_deploys": "deploy history, named before the _<name>_<table> rule; renamed to _deploy_deploys by a migration later",
		},
	})
}
