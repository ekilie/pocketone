package vultr

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"

	"pocketone.dev/ext/llm/providers/oai"
)

// AlignRequest is POST /audio/alignments: forced alignment of known text against its audio.
type AlignRequest struct {
	Model    string // aligner-1
	Audio    []byte
	Filename string // with an extension the provider reads the format from: story.mp3
	Text     string // one line per line of the audio; [Label] lines name sections and aren't timed
	Mode     string // speech, or lyrics (the default: separates the vocal from music first)
	Language string // ISO 639-1, optional
	Extra    map[string]string
}

// Alignment is the answer: when each line and word is said.
type Alignment struct {
	Duration  float64 `json:"duration"`
	Separated bool    `json:"separated"`
	Lines     []struct {
		Index   int     `json:"index"`
		Section int     `json:"section"`
		Tag     *string `json:"tag"`
		Text    string  `json:"text"`
		Start   float64 `json:"start"`
		End     float64 `json:"end"`
		Score   float64 `json:"score"`
		Certain bool    `json:"certain"`
	} `json:"lines"`
	Words []struct {
		Line  int     `json:"line"`
		Text  string  `json:"text"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Score float64 `json:"score"`
	} `json:"words"`
	Unaligned []struct {
		Line   int    `json:"line"`
		Text   string `json:"text"`
		Reason string `json:"reason"`
	} `json:"unaligned"`
	Usage struct {
		Seconds float64  `json:"seconds"`
		Cost    *float64 `json:"cost"`
	} `json:"usage"`
	Raw json.RawMessage `json:"-"`
}

// Align times the words of req.Text in req.Audio.
func (c *Client) Align(ctx context.Context, req AlignRequest) (Alignment, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fields := map[string]string{"model": req.Model, "text": req.Text, "mode": req.Mode, "language": req.Language}
	for k, v := range req.Extra {
		fields[k] = v
	}
	for k, v := range fields {
		if v != "" {
			_ = w.WriteField(k, v)
		}
	}
	name := req.Filename
	if name == "" {
		name = "audio.mp3"
	}
	fw, err := w.CreateFormFile("file", name)
	if err != nil {
		return Alignment{}, err
	}
	_, _ = fw.Write(req.Audio)
	if err := w.Close(); err != nil {
		return Alignment{}, err
	}
	res, err := c.Request(ctx, "POST", "audio/alignments", &buf, w.FormDataContentType())
	if err != nil {
		return Alignment{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return Alignment{}, &oai.Error{Status: res.StatusCode, Message: "reading the answer: " + err.Error()}
	}
	var a Alignment
	if err := json.Unmarshal(raw, &a); err != nil {
		return Alignment{}, &oai.Error{Status: res.StatusCode, Message: "the answer isn't an alignment", Body: string(raw[:min(len(raw), 300)])}
	}
	a.Raw = raw
	return a, nil
}

// CatalogModel is an entry of GET /models/all (the authenticated catalog), which lists models the open /models
// doesn't, such as the aligner: their id and features ("AudioAlignment").
type CatalogModel struct {
	ID        string   `json:"id"`
	Features  []string `json:"features"`
	Languages []string `json:"languages"`
	Raw       json.RawMessage
}

// Catalog lists GET /models/all: every model the key can use, grouped by Vultr's own categories (flattened here).
func (c *Client) Catalog(ctx context.Context) ([]CatalogModel, error) {
	var groups map[string][]json.RawMessage
	if err := c.JSON(ctx, "GET", "models/all", nil, &groups); err != nil {
		return nil, err
	}
	var out []CatalogModel
	for _, items := range groups {
		for _, raw := range items {
			var m CatalogModel
			if json.Unmarshal(raw, &m) == nil && m.ID != "" {
				m.Raw = raw
				out = append(out, m)
			}
		}
	}
	return out, nil
}

// Has reports whether the model has a feature.
func (m CatalogModel) Has(feature string) bool {
	for _, f := range m.Features {
		if f == feature {
			return true
		}
	}
	return false
}
