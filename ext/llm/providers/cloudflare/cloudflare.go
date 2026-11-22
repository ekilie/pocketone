// Package cloudflare drives Cloudflare Workers AI. Chat and embeddings go through its OpenAI-compatible endpoints
// (/ai/v1); the model list is /ai/models/search; pictures, speech and transcription go through the native
// /ai/run/{model}, whose answers come three ways: a JSON envelope ({"result": ...}), raw bytes, or (for FLUX.2) a
// multipart request. Workers AI's own catalog has no music or video models.
package cloudflare

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"pocketone.dev/ext/llm/providers/oai"
)

// Client is a Workers AI client for one account. The embedded oai.Client (BaseURL .../ai/v1) gives ChatStream and
// Embed; Chat is wrapped for Cloudflare's JSON mode.
type Client struct {
	*oai.Client
	native *oai.Client // .../ai
}

// New returns a client for an account id and an API token with Workers AI permission.
func New(accountID, token string) *Client {
	base := "https://api.cloudflare.com/client/v4/accounts/" + url.PathEscape(accountID) + "/ai"
	return NewAt(base, token)
}

// NewAt returns a client for an explicit .../ai base URL (tests, or a gateway).
func NewAt(base, token string) *Client {
	base = strings.TrimRight(base, "/")
	return &Client{Client: &oai.Client{BaseURL: base + "/v1", Key: token}, native: &oai.Client{BaseURL: base, Key: token}}
}

// Chat sends a chat completion. A json_schema response format is sent Cloudflare's way: the schema itself in
// json_schema, without OpenAI's {name, schema} wrapper. (JSON mode doesn't work with streaming.)
func (c *Client) Chat(ctx context.Context, req oai.ChatRequest) (oai.ChatResponse, error) {
	if rf := req.ResponseFormat; rf != nil && rf.JSONSchema != nil {
		extra := map[string]any{}
		for k, v := range req.Extra {
			extra[k] = v
		}
		extra["response_format"] = map[string]any{"type": "json_schema", "json_schema": rf.JSONSchema.Schema}
		req.Extra, req.ResponseFormat = extra, nil
	}
	return c.Client.Chat(ctx, req)
}

// Model is one entry of Cloudflare's model list. ID is what to call (@cf/...); Task is its kind ("Text Generation",
// "Text-to-Image", "Text-to-Speech", "Automatic Speech Recognition", "Text Embeddings", ...).
type Model struct {
	UUID        string
	ID          string
	Description string
	Task        string
	// Properties are the model's properties by id (price, context_window, reasoning, async_queue, ...), raw.
	Properties map[string]json.RawMessage
	Raw        json.RawMessage
}

// Price is one of a model's prices: Unit like "per M input tokens", "per 512 by 512 tile", "per audio minute".
type Price struct {
	Unit     string  `json:"unit"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
}

// Prices are the model's listed prices (some models list none).
func (m Model) Prices() []Price {
	var ps []Price
	_ = json.Unmarshal(m.Properties["price"], &ps)
	return ps
}

// Kind maps the task onto the output kinds the other drivers use: text, image, speech, transcription, embeddings, or
// the task name itself.
func (m Model) Kind() string {
	switch m.Task {
	case "Text Generation":
		return "text"
	case "Text-to-Image":
		return "image"
	case "Text-to-Speech":
		return "speech"
	case "Automatic Speech Recognition":
		return "transcription"
	case "Text Embeddings":
		return "embeddings"
	}
	return m.Task
}

// Models lists the account's available models (not deprecated ones), every page.
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	var out []Model
	for page := 1; page < 50; page++ {
		var env struct {
			Result []json.RawMessage `json:"result"`
		}
		if err := c.native.JSON(ctx, "GET", "models/search?per_page=100&page="+strconv.Itoa(page), nil, &env); err != nil {
			return nil, err
		}
		for _, raw := range env.Result {
			var v struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Task        struct {
					Name string `json:"name"`
				} `json:"task"`
				Properties []struct {
					ID    string          `json:"property_id"`
					Value json.RawMessage `json:"value"`
				} `json:"properties"`
			}
			if json.Unmarshal(raw, &v) != nil || v.Name == "" {
				continue
			}
			m := Model{UUID: v.ID, ID: v.Name, Description: v.Description, Task: v.Task.Name, Properties: map[string]json.RawMessage{}, Raw: raw}
			for _, p := range v.Properties {
				m.Properties[p.ID] = p.Value
			}
			out = append(out, m)
		}
		if len(env.Result) < 100 { // per_page is ignored and always 100
			break
		}
	}
	return out, nil
}

// Answer is what /ai/run returned: Result for a JSON answer (the envelope's result), File for raw bytes.
type Answer struct {
	Result json.RawMessage
	File   oai.File
}

// Run calls /ai/run/{model} with a JSON body (the model's inputs).
func (c *Client) Run(ctx context.Context, model string, input any) (Answer, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return Answer{}, err
	}
	return c.RunRaw(ctx, model, bytes.NewReader(b), "application/json")
}

// RunRaw calls /ai/run/{model} with any body and content type.
func (c *Client) RunRaw(ctx context.Context, model string, body io.Reader, contentType string) (Answer, error) {
	res, err := c.native.Request(ctx, "POST", "run/"+model, body, contentType)
	if err != nil {
		return Answer{}, err
	}
	defer res.Body.Close()
	ct, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "video/") || ct == "application/octet-stream" {
		f, err := oai.ReadFile(res)
		return Answer{File: f}, err
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 256<<20))
	if err != nil {
		return Answer{}, &oai.Error{Status: res.StatusCode, Message: "reading the answer: " + err.Error()}
	}
	var env struct {
		Success *bool           `json:"success"`
		Result  json.RawMessage `json:"result"`
		Errors  []struct {
			Code    any    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return Answer{}, &oai.Error{Status: res.StatusCode, Message: "the answer isn't JSON", Body: string(raw[:min(len(raw), 300)])}
	}
	if env.Success != nil && !*env.Success {
		msg := "the call failed"
		if len(env.Errors) > 0 {
			msg = env.Errors[0].Message
		}
		return Answer{}, &oai.Error{Status: res.StatusCode, Message: msg}
	}
	if len(env.Result) == 0 { // a model that answers without the envelope
		env.Result = raw
	}
	return Answer{Result: env.Result}, nil
}

// ImageRequest is a text-to-image call. Extra holds the model's other inputs (guidance, negative_prompt, ...).
type ImageRequest struct {
	Model  string
	Prompt string
	Width  int
	Height int
	Steps  int
	Seed   int
	Extra  map[string]any
}

// Image generates one picture. FLUX.2 models are sent as multipart form data, the rest as JSON; the answer is
// decoded from base64 ({"image": ...}) or taken as raw bytes.
func (c *Client) Image(ctx context.Context, req ImageRequest) (oai.File, error) {
	in := map[string]any{"prompt": req.Prompt}
	for k, v := range map[string]int{"width": req.Width, "height": req.Height, "steps": req.Steps, "seed": req.Seed} {
		if v != 0 {
			in[k] = v
		}
	}
	for k, v := range req.Extra {
		in[k] = v
	}
	var a Answer
	var err error
	if strings.Contains(req.Model, "/flux-2") {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for k, v := range in {
			_ = w.WriteField(k, fmt.Sprint(v))
		}
		_ = w.Close()
		a, err = c.RunRaw(ctx, req.Model, &buf, w.FormDataContentType())
	} else {
		a, err = c.Run(ctx, req.Model, in)
	}
	if err != nil {
		return oai.File{}, err
	}
	return fileFrom(a, "image")
}

// fileFrom is a's raw file, or the base64 in its result's field.
func fileFrom(a Answer, field string) (oai.File, error) {
	if len(a.File.Data) > 0 {
		return a.File, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(a.Result, &m); err != nil {
		return oai.File{}, &oai.Error{Message: "the answer has no " + field}
	}
	var s string
	if err := json.Unmarshal(m[field], &s); err != nil || s == "" {
		return oai.File{}, &oai.Error{Message: "the answer has no " + field, Body: string(a.Result[:min(len(a.Result), 300)])}
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return oai.File{URL: s}, nil
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return oai.File{}, &oai.Error{Message: field + " isn't base64: " + err.Error()}
	}
	return oai.File{Data: b, ContentType: http.DetectContentType(b)}, nil
}

// SpeechRequest is a text-to-speech call. Voice is the Deepgram aura "speaker"; Encoding like mp3 (default), opus,
// flac. Extra holds the model's other inputs.
type SpeechRequest struct {
	Model    string
	Text     string
	Voice    string
	Encoding string
	Extra    map[string]any
}

// Speech turns text into audio: raw bytes, or base64 in {"audio": ...} (older models).
func (c *Client) Speech(ctx context.Context, req SpeechRequest) (oai.File, error) {
	in := map[string]any{"text": req.Text}
	if strings.Contains(req.Model, "melotts") {
		in = map[string]any{"prompt": req.Text}
	}
	if req.Voice != "" {
		in["speaker"] = req.Voice
	}
	if req.Encoding != "" {
		in["encoding"] = req.Encoding
	}
	for k, v := range req.Extra {
		in[k] = v
	}
	a, err := c.Run(ctx, req.Model, in)
	if err != nil {
		return oai.File{}, err
	}
	f, err := fileFrom(a, "audio")
	if err == nil && (f.ContentType == "" || f.ContentType == "application/octet-stream") {
		f.ContentType = "audio/mpeg"
	}
	return f, err
}

// Transcribe turns audio into text. whisper-large-v3-turbo takes base64 in JSON; other models take the audio as the
// body.
func (c *Client) Transcribe(ctx context.Context, model string, audio []byte, language string) (oai.Transcript, error) {
	var a Answer
	var err error
	if strings.Contains(model, "large-v3") {
		in := map[string]any{"audio": base64.StdEncoding.EncodeToString(audio)}
		if language != "" {
			in["language"] = language
		}
		a, err = c.Run(ctx, model, in)
	} else {
		a, err = c.RunRaw(ctx, model, bytes.NewReader(audio), "application/octet-stream")
	}
	if err != nil {
		return oai.Transcript{}, err
	}
	var v struct {
		Text    string `json:"text"`
		Results struct {
			Channels []struct {
				Alternatives []struct {
					Transcript string `json:"transcript"`
				} `json:"alternatives"`
			} `json:"channels"`
		} `json:"results"`
	}
	if err := json.Unmarshal(a.Result, &v); err != nil {
		return oai.Transcript{}, &oai.Error{Message: "the answer isn't a transcript"}
	}
	text := v.Text
	if text == "" && len(v.Results.Channels) > 0 && len(v.Results.Channels[0].Alternatives) > 0 { // Deepgram nova
		text = v.Results.Channels[0].Alternatives[0].Transcript
	}
	return oai.Transcript{Text: text, Raw: a.Result}, nil
}
