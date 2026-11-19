package agentchat_test

import (
	"testing"

	"pocketone.dev/ext/agent"
	agentchat "pocketone.dev/ext/agent-chat"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return agentchat.New() },
		conformance.Options{Deps: []extkit.Extension{agent.New()}})
}
