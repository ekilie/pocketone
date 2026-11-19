package agentchat_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/agent"
	agentchat "github.com/pockethost/pocketone/ext/agent-chat"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return agentchat.New() },
		conformance.Options{Deps: []extkit.Extension{agent.New()}})
}
