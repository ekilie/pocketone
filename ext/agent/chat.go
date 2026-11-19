package agent

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// bindChatJS adds $agent's chat methods when a provider of agent.chat/v1 runs on app (filled in with ext/agent-chat).
func bindChatJS(app core.App, rt js.Runtime, n js.Object) {}
