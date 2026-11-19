package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// OLLAMA_TOKCHECK=qwen3:0.6b GGUF=... go test -run TestTokenCountsVsOllama
func TestTokenCountsVsOllama(t *testing.T) {
	model, p := os.Getenv("OLLAMA_TOKCHECK"), os.Getenv("GGUF")
	if model == "" || p == "" {
		t.Skip()
	}
	m, err := Load(p, LoadOptions{Threads: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	cases := []string{
		"Hello world", "  leading spaces and trailing   ", "tabs\tand\nnewlines\n\n\nthree", "don't I'LL we've they'Re",
		"numbers 1234567 3.14159 1e10 0x1F", "émigré naïve café 北京欢迎你 こんにちは 🙂🚀 👨‍👩‍👧",
		"func main() {\n\tfmt.Println(\"hi\")\n}\n", "<|im_start|>user\nhi<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n",
		"a  b   c    d\r\n\r\ne", "!!!??? ... --- ### $$$", "MixedCASE camelCase snake_case kebab-case", "x\n \n  \ny",
		"Ünïcödé ₂₃ ½ ⅷ ٣ 𝔘𝔫𝔦", "   \n", "end with space ",
	}
	bad := 0
	for _, c := range cases {
		body, _ := json.Marshal(map[string]any{"model": model, "prompt": c, "raw": true, "stream": false,
			"options": map[string]any{"num_predict": 1, "temperature": 0}})
		cl := http.Client{Timeout: 60 * time.Second}
		resp, err := cl.Post("http://127.0.0.1:11434/api/generate", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		var r struct {
			PromptEvalCount int `json:"prompt_eval_count"`
		}
		json.NewDecoder(resp.Body).Decode(&r)
		resp.Body.Close()
		ids := m.Tokenize(c)
		ok := len(ids) == r.PromptEvalCount
		if !ok {
			bad++
		}
		fmt.Printf("%v ours=%d ollama=%d %q\n", ok, len(ids), r.PromptEvalCount, c)
		if got := m.Detokenize(ids); got != c {
			t.Errorf("round trip: %q -> %q", c, got)
		}
	}
	if bad > 0 {
		t.Errorf("%d token count mismatches", bad)
	}
}
