package bulkmail_test

import (
	"testing"

	"pocketone.dev/ext/bulkmail"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return bulkmail.New(bulkmail.Config{}) })
}
