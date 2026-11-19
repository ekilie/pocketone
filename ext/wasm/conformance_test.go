package wasm_test

import (
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/wasm"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return wasm.New(wasm.Config{}) })
}
