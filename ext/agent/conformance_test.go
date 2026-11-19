package agent_test

import (
	"testing"

	"pocketone.dev/ext/agent"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return agent.New() })
}
