// Package openrouter drives OpenRouter (https://openrouter.ai/api/v1). Most of it is OpenAI-shaped (package oai):
// chat, speech, transcription, embeddings. Its own parts: a model list that covers every kind only when asked
// (output_modalities=all) and names the endpoint that serves each model; pictures on /images (or inside a chat answer
// for chat-capable image models); video as an async job, the same shape as Vultr's.
package openrouter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"pocketone.dev/ext/llm/providers/oai"
)

// DefaultBaseURL is OpenRouter's API.
const DefaultBaseURL = "https://openrouter.ai/api/v1"

// Client is an OpenRouter client. The embedded oai.Client gives Chat, ChatStream, Transcribe and Embed. Set
// Header (X-OpenRouter-Title, HTTP-Referer) for attribution if wanted; none is sent by default.
type Client struct {
	*oai.Client
}

// New returns a client for key. baseURL "" is DefaultBaseURL.
func New(baseURL, key string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{&oai.Client{BaseURL: baseURL, Key: key}}
}

// Model is one entry of OpenRouter's model list. Pricing values are USD decimal strings per token, image or request
// ("0" free, "-1" dynamic routers). Endpoints are the API paths that serve the model, like /api/v1/images.
type Model struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	ContextLength int               `json:"context_length"`
	Architecture  Architecture      `json:"architecture"`
	Pricing       map[string]any    `json:"pricing"`
	Params        []string          `json:"supported_parameters"`
	Voices        []string          `json:"supported_voices"`
	Endpoints     []string          `json:"endpoints"`
	Reasoning     *ReasoningSupport `json:"reasoning"`
	Raw           json.RawMessage   `json:"-"`
}

// Architecture is a model's input and output kinds: text, image, audio, file, video; outputs also speech,
// transcription, embeddings, rerank.
type Architecture struct {
	Modality string   `json:"modality"`
	Inputs   []string `json:"input_modalities"`
	Outputs  []string `json:"output_modalities"`
}

// ReasoningSupport says how a model thinks: Mandatory, DefaultEnabled, the efforts it supports.
type ReasoningSupport struct {
	Mandatory        bool     `json:"mandatory"`
	DefaultEnabled   bool     `json:"default_enabled"`
	SupportedEfforts []string `json:"supported_efforts"`
	DefaultEffort    string   `json:"default_effort"`
}

// Makes reports whether the model makes outputs of kind t.
func (m Model) Makes(t string) bool {
	for _, o := range m.Architecture.Outputs {
		if o == t {
			return true
		}
	}
	return false
}

// Serves reports whether path (like "/api/v1/images") serves the model.
func (m Model) Serves(path string) bool {
	for _, e := range m.Endpoints {
		if e == path {
			return true
		}
	}
	return false
}

// Models lists models making outputs of the given kinds ("all" for every kind; "" is OpenRouter's default, text only).
func (c *Client) Models(ctx context.Context, outputs string) ([]Model, error) {
	path := "models"
	if outputs != "" {
		path += "?output_modalities=" + url.QueryEscape(outputs)
	}
	var v struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := c.JSON(ctx, "GET", path, nil, &v); err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(v.Data))
	for _, raw := range v.Data {
		var m Model
		if json.Unmarshal(raw, &m) != nil || m.ID == "" {
			continue
		}
		m.Raw = raw
		out = append(out, m)
	}
	return out, nil
}

// Images generates pictures on /images. A b64_json answer (the default) is decoded.
func (c *Client) Images(ctx context.Context, req oai.ImageRequest) (oai.ImageResponse, error) {
	return c.ImagesAt(ctx, "images", req)
}

// ChatImages asks a chat-capable image model for pictures (modalities image+text) and returns the answer's text and
// pictures, decoded from their data URLs.
func (c *Client) ChatImages(ctx context.Context, req oai.ChatRequest) (oai.ChatResponse, []oai.File, error) {
	extra := map[string]any{"modalities": []string{"image", "text"}}
	for k, v := range req.Extra {
		extra[k] = v
	}
	req.Extra = extra
	r, err := c.Chat(ctx, req)
	if err != nil {
		return r, nil, err
	}
	files, err := MessageImages(r)
	return r, files, err
}

// MessageImages decodes the pictures in a chat answer's message.images[].image_url.url (data URLs, or plain URLs left
// as File.URL).
func MessageImages(r oai.ChatResponse) ([]oai.File, error) {
	if len(r.Choices) == 0 {
		return nil, nil
	}
	var m struct {
		Images []struct {
			ImageURL struct {
				URL string `json:"url"`
			} `json:"image_url"`
		} `json:"images"`
	}
	if err := json.Unmarshal(r.Choices[0].Message.Raw, &m); err != nil {
		return nil, nil
	}
	var out []oai.File
	for _, im := range m.Images {
		f, err := DataURL(im.ImageURL.URL)
		if err != nil {
			return out, err
		}
		out = append(out, f)
	}
	return out, nil
}

// DataURL decodes a data:<type>;base64,<data> URL; any other URL is returned as File.URL.
func DataURL(u string) (oai.File, error) {
	rest, ok := strings.CutPrefix(u, "data:")
	if !ok {
		return oai.File{URL: u}, nil
	}
	meta, data, ok := strings.Cut(rest, ",")
	if !ok || !strings.HasSuffix(meta, ";base64") {
		return oai.File{}, &oai.Error{Message: "not a base64 data URL"}
	}
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return oai.File{}, &oai.Error{Message: "bad base64 in data URL: " + err.Error()}
	}
	return oai.File{Data: b, ContentType: strings.TrimSuffix(meta, ";base64")}, nil
}

// Speech turns text into audio on /audio/speech. OpenRouter's default format is raw PCM; this asks for mp3 unless the
// request says otherwise.
func (c *Client) Speech(ctx context.Context, req oai.SpeechRequest) (oai.File, error) {
	if req.ResponseFormat == "" {
		req.ResponseFormat = "mp3"
	}
	return c.Client.Speech(ctx, req)
}

// VideoRequest is POST /videos. Extra holds the other fields (frame_images, input_references, seed, ...).
type VideoRequest struct {
	Model         string         `json:"model"`
	Prompt        string         `json:"prompt"`
	Duration      int            `json:"duration,omitempty"`
	Resolution    string         `json:"resolution,omitempty"`
	AspectRatio   string         `json:"aspect_ratio,omitempty"`
	GenerateAudio bool           `json:"generate_audio,omitempty"`
	Extra         map[string]any `json:"-"`
}

// Video starts a video job; poll it with Wait and download with Content.
func (c *Client) Video(ctx context.Context, req VideoRequest) (oai.Job, error) {
	return c.StartJob(ctx, "videos", req, req.Extra)
}

// Wait polls a video job (OpenRouter suggests every 10–30 s).
func (c *Client) Wait(ctx context.Context, j oai.Job, interval time.Duration) (oai.Job, error) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return c.WaitJob(ctx, "videos", j, interval)
}

// Content downloads output index of a finished video job.
func (c *Client) Content(ctx context.Context, id string, index int) (oai.File, error) {
	return c.JobContent(ctx, "videos", id, index)
}
