package mothership_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/mothership"
)

func TestConformance(t *testing.T) {
	on := true
	conformance.Run(t, func() extkit.Extension { return &mothership.Schema{Enabled: &on, Apex: "example.test"} }, conformance.Options{
		AllowCollections: map[string]string{
			"edges":     "the fleet's records, edited by the operator in the dashboard; named for what they hold",
			"instances": "the fleet's records, edited by the operator in the dashboard; named for what they hold",
		},
	})
	conformance.Run(t, func() extkit.Extension { return &mothership.Server{Enabled: &on} })
	conformance.Run(t, func() extkit.Extension { return mothership.NewServer() }) // off without the section
}
