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

// A richer schema on the real engine: several fields, a number and an array, all filled in from text.
func TestRealModelExtract(t *testing.T) {
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
		System: "Extract the order from the email.",
		Prompt: "Hi, this is Dana Reyes. Please send 3 blue mugs and 2 red plates to our Austin office. Thanks!",
		Schema: json.RawMessage(`{"type":"object","properties":{
			"name":{"type":"string"},
			"city":{"type":"string"},
			"items":{"type":"array","maxItems":5,"items":{"type":"object","properties":{"what":{"type":"string"},"count":{"type":"integer"}},"required":["what","count"]}}
		},"required":["name","city","items"]}`),
		MaxTokens: 120,
	})
	t.Logf("text %q usage %+v stop %s", res.Text, res.Usage, res.StopReason)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Name, City string
		Items      []struct {
			What  string
			Count int
		}
	}
	if err := json.Unmarshal(res.Data, &out); err != nil {
		t.Fatalf("data: %v", err)
	}
	if out.City != "Austin" || len(out.Items) != 2 || out.Items[0].Count != 3 {
		t.Fatalf("extracted: %+v", out)
	}
}
