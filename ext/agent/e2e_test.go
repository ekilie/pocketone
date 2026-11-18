package agent

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// The real engine end to end through agent/v1: AGENT_GGUF=<a Qwen3 .gguf> go test -run TestRealModel -v
func TestRealModel(t *testing.T) {
	path := os.Getenv("AGENT_GGUF")
	if path == "" {
		t.Skip("set AGENT_GGUF to a Qwen3 GGUF file")
	}
	runnersMu.Lock()
	runners = map[string]*runner{}
	runnersMu.Unlock()
	withSection(t, `{"model": "`+path+`", "threads": 4}`)
	app := testApp(t)
	if err := New().Register(app, nil); err != nil {
		t.Fatal(err)
	}
	a := agentv1.Contract.Must(app)
	waitReady(t, a)
	res, err := a.Complete(context.Background(), extkit.Internal, agentv1.CompleteInput{
		System:    "Label the support ticket.",
		Prompt:    "The app crashes with a white screen every time I open settings.",
		Schema:    json.RawMessage(`{"type":"object","properties":{"label":{"enum":["bug","billing","question"]}},"required":["label"]}`),
		MaxTokens: 40,
	})
	t.Logf("text %q data %s usage %+v stop %s", res.Text, res.Data, res.Usage, res.StopReason)
	if err != nil {
		t.Fatal(err)
	}
	var out struct{ Label string }
	if err := json.Unmarshal(res.Data, &out); err != nil || out.Label != "bug" {
		t.Fatalf("label: %+v %v", out, err)
	}
}
