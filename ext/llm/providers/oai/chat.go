package oai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// Message is one chat message. Content is a string or a list of content parts (text, image_url, input_audio...).
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ChatRequest is a chat completion request. Extra holds provider-specific fields (reasoning, modalities, ...).
type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	Stop           []string        `json:"stop,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	StreamOptions  *StreamOptions  `json:"stream_options,omitempty"`
	Extra          map[string]any  `json:"-"`
}

// ResponseFormat asks for JSON: Type "json_object", or "json_schema" with JSONSchema.
type ResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// JSONSchema names a schema for ResponseFormat.
type JSONSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict,omitempty"`
}

// StreamOptions asks a stream to end with usage.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ChatResponse is a chat completion. Raw is the whole answer (for a stream: the last chunk), for fields a driver reads
// itself.
type ChatResponse struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Choices []Choice        `json:"choices"`
	Usage   Usage           `json:"usage"`
	Raw     json.RawMessage `json:"-"`
}

// Choice is one answer.
type Choice struct {
	Message      AnswerMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// AnswerMessage is the model's message. Reasoning is the model's thinking when the provider returns it
// (reasoning_content or reasoning). Extra keeps the message's raw JSON for provider fields (images, audio).
type AnswerMessage struct {
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Reasoning string          `json:"-"`
	Raw       json.RawMessage `json:"-"`
}

func (m *AnswerMessage) UnmarshalJSON(b []byte) error {
	var v struct {
		Role             string `json:"role"`
		Content          any    `json:"content"`
		Reasoning        string `json:"reasoning"`
		ReasoningContent string `json:"reasoning_content"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	m.Role, m.Content, m.Raw = v.Role, contentText(v.Content), append(json.RawMessage(nil), b...)
	m.Reasoning = v.ReasoningContent
	if m.Reasoning == "" {
		m.Reasoning = v.Reasoning
	}
	return nil
}

// contentText is a message content's text: the string itself, or its text parts joined.
func contentText(c any) string {
	switch v := c.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, p := range v {
			if m, ok := p.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
		return b.String()
	}
	return ""
}

// Usage is a call's token counts. Cost is in US dollars when the provider reports it.
type Usage struct {
	PromptTokens        int      `json:"prompt_tokens"`
	CompletionTokens    int      `json:"completion_tokens"`
	TotalTokens         int      `json:"total_tokens"`
	Cost                *float64 `json:"cost,omitempty"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

// Text is the first choice's content.
func (r ChatResponse) Text() string {
	if len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}

// Chat sends a chat completion and waits for the whole answer.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	req.Stream, req.StreamOptions = false, nil
	body, err := withExtra(req, req.Extra)
	if err != nil {
		return ChatResponse{}, err
	}
	res, err := c.Request(ctx, "POST", "chat/completions", bytes.NewReader(body), "application/json")
	if err != nil {
		return ChatResponse{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return ChatResponse{}, &Error{Status: res.StatusCode, Message: "reading the answer: " + err.Error()}
	}
	var out ChatResponse
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return ChatResponse{}, &Error{Status: res.StatusCode, Message: "the answer isn't a chat completion", Body: clip(string(raw), 300)}
	}
	out.Raw = raw
	return out, nil
}

// Delta is one streamed piece: text and, when the provider streams it, thinking. Raw is the chunk's delta object.
type Delta struct {
	Text      string
	Reasoning string
	Raw       json.RawMessage
}

// ChatStream sends a streamed chat completion, calling onDelta for each piece (return false to stop), and returns the
// assembled answer with usage when the provider sends it.
func (c *Client) ChatStream(ctx context.Context, req ChatRequest, onDelta func(Delta) bool) (ChatResponse, error) {
	req.Stream, req.StreamOptions = true, &StreamOptions{IncludeUsage: true}
	body, err := withExtra(req, req.Extra)
	if err != nil {
		return ChatResponse{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	res, err := c.Request(ctx, "POST", "chat/completions", bytes.NewReader(body), "application/json")
	if err != nil {
		return ChatResponse{}, err
	}
	defer res.Body.Close()
	return readStream(res.Body, onDelta)
}

func readStream(r io.Reader, onDelta func(Delta) bool) (ChatResponse, error) {
	var out ChatResponse
	var text, reasoning strings.Builder
	finish := ""
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue // blank lines, "event:" lines, ": keepalive" comments
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ch struct {
			ID      string `json:"id"`
			Model   string `json:"model"`
			Choices []struct {
				Delta        json.RawMessage `json:"delta"`
				FinishReason *string         `json:"finish_reason"`
			} `json:"choices"`
			Usage *Usage          `json:"usage"`
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal([]byte(data), &ch) != nil {
			continue
		}
		if len(ch.Error) > 0 && string(ch.Error) != "null" {
			var e struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal(ch.Error, &e)
			return out, &Error{Message: "the stream failed: " + e.Message, Body: clip(data, 300)}
		}
		if ch.ID != "" {
			out.ID = ch.ID
		}
		if ch.Model != "" {
			out.Model = ch.Model
		}
		if ch.Usage != nil {
			out.Usage = *ch.Usage
		}
		out.Raw = json.RawMessage(data)
		for _, choice := range ch.Choices {
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				finish = *choice.FinishReason
			}
			var d struct {
				Content          any    `json:"content"`
				Reasoning        string `json:"reasoning"`
				ReasoningContent string `json:"reasoning_content"`
			}
			if len(choice.Delta) == 0 || json.Unmarshal(choice.Delta, &d) != nil {
				continue
			}
			piece := Delta{Text: contentText(d.Content), Reasoning: d.ReasoningContent, Raw: choice.Delta}
			if piece.Reasoning == "" {
				piece.Reasoning = d.Reasoning
			}
			if piece.Text == "" && piece.Reasoning == "" && len(choice.Delta) <= 2 {
				continue
			}
			text.WriteString(piece.Text)
			reasoning.WriteString(piece.Reasoning)
			if onDelta != nil && !onDelta(piece) {
				out.Choices = []Choice{{Message: AnswerMessage{Role: "assistant", Content: text.String(), Reasoning: reasoning.String()}, FinishReason: "cancelled"}}
				return out, nil
			}
		}
	}
	if err := sc.Err(); err != nil {
		return out, &Error{Message: "reading the stream: " + err.Error()}
	}
	out.Choices = []Choice{{Message: AnswerMessage{Role: "assistant", Content: text.String(), Reasoning: reasoning.String()}, FinishReason: finish}}
	return out, nil
}
