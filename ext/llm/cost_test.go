package llm

import (
	"math"
	"testing"

	llmc "pocketone.dev/ext/contracts/llm"
)

// The units are the providers' own, as their model lists give them (see the drivers' models()).
func TestEstimate(t *testing.T) {
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-12 }
	cases := []struct {
		name   string
		prices []llmc.Price
		u      llmc.Usage
		x      used
		want   float64
		ok     bool
	}{
		{"vultr: output per token only", []llmc.Price{{Unit: "completion per token", USD: 3e-6}},
			llmc.Usage{InputTokens: 787, OutputTokens: 190}, used{}, 190 * 3e-6, true},
		{"cloudflare: per M input and output", []llmc.Price{{Unit: "per M input tokens", USD: 0.293}, {Unit: "per M output tokens", USD: 2.253}},
			llmc.Usage{InputTokens: 1000, OutputTokens: 500}, used{}, 1000*0.293/1e6 + 500*2.253/1e6, true},
		{"openrouter units: cached input billed at its own price", []llmc.Price{{Unit: "1M cached input tokens", USD: 0.075},
			{Unit: "1M input tokens", USD: 0.15}, {Unit: "1M output tokens", USD: 0.6}},
			llmc.Usage{InputTokens: 949, CachedTokens: 256, OutputTokens: 117}, used{}, 693*0.15/1e6 + 256*0.075/1e6 + 117*0.6/1e6, true},
		{"cached tokens at the input price when no cached price is listed", []llmc.Price{{Unit: "1M input tokens", USD: 1}},
			llmc.Usage{InputTokens: 100, CachedTokens: 40}, used{}, 100 / 1e6, true},
		{"per picture", []llmc.Price{{Unit: "image", USD: 0.04}}, llmc.Usage{}, used{files: 2}, 0.08, true},
		{"per second of audio", []llmc.Price{{Unit: "completion per second", USD: 0.002}}, llmc.Usage{Seconds: 10}, used{}, 0.02, true},
		{"per million characters", []llmc.Price{{Unit: "per M characters", USD: 15}}, llmc.Usage{}, used{chars: 2000}, 2000 * 15 / 1e6, true},
		{"a unit it can't count", []llmc.Price{{Unit: "megapixel", USD: 0.01}}, llmc.Usage{InputTokens: 5}, used{files: 1}, 0, false},
		{"no prices", nil, llmc.Usage{InputTokens: 5, OutputTokens: 5}, used{}, 0, false},
	}
	for _, c := range cases {
		got, ok := estimate(c.prices, c.u, c.x)
		if ok != c.ok || !near(got, c.want) {
			t.Errorf("%s: got %v %v, want %v %v", c.name, got, ok, c.want, c.ok)
		}
	}
}
