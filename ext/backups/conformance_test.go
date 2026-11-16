package backups_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/backups"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return backups.New() })
}
