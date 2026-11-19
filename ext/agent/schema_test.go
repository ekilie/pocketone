package agent

import (
	"encoding/json"
	"testing"

	"github.com/pockethost/pocketone/ext/agent/engine"
)

func TestSchemaFit(t *testing.T) {
	label := `{"type":"object","properties":{"label":{"enum":["bug","billing","question"]},"score":{"type":"integer"},"tags":{"type":"array","items":{"type":"string"},"maxItems":2}},"required":["label"]}`
	cases := []struct {
		schema, text string
		want         engine.Fit
	}{
		{label, ``, engine.FitPrefix},
		{label, `{`, engine.FitPrefix},
		{label, `{"la`, engine.FitPrefix},
		{label, `{"lx`, engine.FitNo},                // not a property
		{label, `{"label": "b`, engine.FitPrefix},    // bug or billing
		{label, `{"label": "bi`, engine.FitPrefix},   // billing
		{label, `{"label": "bx`, engine.FitNo},       // not in the enum
		{label, `{"label": "bug"}`, engine.FitDone},  // complete
		{label, `{"label": "bug"} `, engine.FitDone}, // trailing space
		{label, `{"label": "bug"}x`, engine.FitNo},   // nothing after
		{label, `{}`, engine.FitNo},                  // label is required
		{label, `{"score": 3}`, engine.FitNo},        // still required
		{label, `{"score": 3, "label": "question"}`, engine.FitDone},
		{label, `{"score": 3.`, engine.FitNo},                         // an integer
		{label, `{"score": 12`, engine.FitPrefix},                     // inside an object: more must follow
		{label, `{"label": "bug", "label"`, engine.FitNo},             // twice
		{label, `{"label": "bug", "tags": ["a", "b", `, engine.FitNo}, // maxItems 2
		{label, `{"label": "bug", "tags": ["a", "b"], "score": 1}`, engine.FitDone},
		{label, `{"label": "bug", "tags": ["a", "b"], "score": 1,`, engine.FitNo}, // every property written
		{label, "{\n\n\n\n\n\"label\"", engine.FitNo},                             // too much whitespace
		{label, "```json", engine.FitNo},                                          // no fences
		{`{"type":"number"}`, `12`, engine.FitComplete},                           // could go on
		{`{"type":"number"}`, `12.5e`, engine.FitPrefix},
		{`{"type":"number"}`, `-`, engine.FitPrefix},
		{`{"type":"boolean"}`, `tr`, engine.FitPrefix},
		{`{"type":"boolean"}`, `true`, engine.FitDone},
		{`{"type":"string"}`, `"a\u00`, engine.FitPrefix},
		{`{"type":"string"}`, `"a\q`, engine.FitNo},
		{`{"anyOf":[{"type":"string"},{"type":"null"}]}`, `nu`, engine.FitPrefix},
		{`{"anyOf":[{"type":"string"},{"type":"null"}]}`, `null`, engine.FitDone},
		{`{"enum":[1,10]}`, `1`, engine.FitPrefix},
		{`{"enum":[1,10]}`, `10`, engine.FitDone},
		{`{"type":"object"}`, `{"anything": [1, {"x": null}]}`, engine.FitDone},
	}
	for _, c := range cases {
		n, err := compileSchema(json.RawMessage(c.schema))
		if err != nil {
			t.Fatal(err)
		}
		if got := n.fit(c.text); got != c.want {
			t.Errorf("%s / %q: %v, want %v", c.schema, c.text, got, c.want)
		}
	}
}
