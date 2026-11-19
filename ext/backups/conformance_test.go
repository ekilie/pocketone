package backups_test

import (
	"testing"

	"pocketone.dev/ext/backups"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return backups.New() })
}
