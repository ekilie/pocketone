package main

import "testing"

func TestTablePrefix(t *testing.T) {
	if got := tablePrefix("agent-chat"); got != "_agent_chat_" {
		t.Fatal(got)
	}
}
