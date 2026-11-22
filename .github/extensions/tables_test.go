package main

import "testing"

func TestTablePrefix(t *testing.T) {
	if got := tablePrefix("llm-chat"); got != "_llm_chat_" {
		t.Fatal(got)
	}
}
