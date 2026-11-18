package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// apiBackend answers through an OpenAI-compatible chat completions API: OpenAI, OpenRouter, Groq, Ollama, llama-server,
// or anything else that speaks it. Endpoint is the API's base URL including its version ("https://api.openai.com/v1",
// "http://localhost:11434/v1").
type apiBackend struct {
	Endpoint string
	Key      string
	Model    string
	Timeout  time.Duration
	// Client defaults to http.DefaultClient, so a multisite host's egress policy applies (no private addresses).
	Client *http.Client
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	Stop           []string        `json:"stop,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string          `json:"type"`
	JSONSchema *jsonSchemaSpec `json:"json_schema,omitempty"`
}

type jsonSchemaSpec struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (b *apiBackend) complete(ctx context.Context, in agentv1.CompleteInput) (agentv1.CompleteResult, error) {
	if b.Endpoint == "" {
		return agentv1.CompleteResult{}, agentv1.ErrNotConfigured
	}
	model := b.Model
	if model == "" {
		model = in.Model
	}
	req := chatRequest{Model: model, Messages: toChat(in), MaxTokens: in.MaxTokens, Temperature: in.Temperature, Stop: in.Stop}
	if len(in.Schema) > 0 {
		req.ResponseFormat = &responseFormat{Type: "json_schema", JSONSchema: &jsonSchemaSpec{Name: "answer", Schema: in.Schema}}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return agentv1.CompleteResult{}, err
	}
	timeout := b.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(b.Endpoint, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return agentv1.CompleteResult{}, &extkit.Error{Code: "bad_endpoint", Message: err.Error(), Status: 400}
	}
	hr.Header.Set("Content-Type", "application/json")
	if b.Key != "" {
		hr.Header.Set("Authorization", "Bearer "+b.Key)
	}
	client := b.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(hr)
	if err != nil {
		return agentv1.CompleteResult{}, &extkit.Error{Code: "upstream", Message: "the model API didn't answer: " + err.Error(), Status: 502}
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return agentv1.CompleteResult{}, &extkit.Error{Code: "upstream", Message: err.Error(), Status: 502}
	}
	var cr chatResponse
	jerr := json.Unmarshal(raw, &cr)
	if res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if jerr == nil && cr.Error != nil && cr.Error.Message != "" {
			msg = cr.Error.Message
		}
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		return agentv1.CompleteResult{}, &extkit.Error{Code: "upstream", Message: fmt.Sprintf("the model API said %d: %s", res.StatusCode, msg), Status: 502}
	}
	if jerr != nil || len(cr.Choices) == 0 {
		return agentv1.CompleteResult{}, &extkit.Error{Code: "upstream", Message: "the model API's answer wasn't a chat completion", Status: 502}
	}
	out := agentv1.CompleteResult{
		Text: cr.Choices[0].Message.Content, Model: cr.Model, Backend: "app",
		StopReason: finishReason(cr.Choices[0].FinishReason),
		Usage:      agentv1.Usage{InputTokens: cr.Usage.PromptTokens, OutputTokens: cr.Usage.CompletionTokens},
	}
	if out.Model == "" {
		out.Model = model
	}
	if len(in.Schema) > 0 {
		data, err := extractJSON(out.Text)
		if err != nil {
			return out, &extkit.Error{Code: "bad_answer", Message: "the model's answer isn't JSON: " + err.Error(), Status: 502}
		}
		out.Data = data
	}
	return out, nil
}

func finishReason(r string) string {
	switch r {
	case "length":
		return "length"
	case "":
		return ""
	}
	return "stop"
}

// toChat turns the contract's input into chat messages: the system prompt first, then the conversation, then Prompt
// as the last user turn.
func toChat(in agentv1.CompleteInput) []chatMessage {
	var out []chatMessage
	if sys := systemPrompt(in); sys != "" {
		out = append(out, chatMessage{Role: "system", Content: sys})
	}
	for _, m := range in.Messages {
		out = append(out, chatMessage{Role: m.Role, Content: m.Content})
	}
	if in.Prompt != "" {
		out = append(out, chatMessage{Role: "user", Content: in.Prompt})
	}
	return out
}

// systemPrompt is in.System, plus the JSON schema when one is asked for (so models without structured output still
// know the shape).
func systemPrompt(in agentv1.CompleteInput) string {
	if len(in.Schema) == 0 {
		return in.System
	}
	s := strings.TrimSpace(in.System)
	if s != "" {
		s += "\n\n"
	}
	return s + "Answer with only a JSON value matching this JSON schema, and nothing else:\n" + string(in.Schema)
}

// extractJSON finds the JSON value in a model's answer: the whole text, or inside a ``` fence, or from the first { or [
// to the matching last } or ].
func extractJSON(text string) (json.RawMessage, error) {
	t := strings.TrimSpace(text)
	if i := strings.Index(t, "```"); i >= 0 {
		rest := t[i+3:]
		rest = strings.TrimPrefix(rest, "json")
		if j := strings.Index(rest, "```"); j >= 0 {
			t = strings.TrimSpace(rest[:j])
		}
	}
	if json.Valid([]byte(t)) {
		return json.RawMessage(t), nil
	}
	for _, pair := range [][2]string{{"{", "}"}, {"[", "]"}} {
		i, j := strings.Index(t, pair[0]), strings.LastIndex(t, pair[1])
		if i >= 0 && j > i && json.Valid([]byte(t[i:j+1])) {
			return json.RawMessage(t[i : j+1]), nil
		}
	}
	return nil, fmt.Errorf("no JSON value in %q", clip(text, 120))
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
