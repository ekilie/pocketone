package llm

import (
	"context"
	"regexp"
	"strings"

	llmc "pocketone.dev/ext/contracts/llm"
)

// Cost: every result's usage.costUSD, filled in when the provider didn't report one, from the model's listed prices
// (the provider's model list, cached for listTTL) and what the call used. usage.costEstimated marks it.

// used is what a call used beyond its usage's tokens and seconds, for prices per picture, character or request.
type used struct {
	files int // pictures made
	chars int // characters read aloud
}

// fillCost sets u.CostUSD from t's listed prices when the provider left it empty. It never fails a call: without a
// list, a listed price or a unit it can count, the cost stays empty.
func (r *router) fillCost(ctx context.Context, t target, u *llmc.Usage, x used) {
	if u == nil || u.CostUSD != nil {
		return
	}
	ms, err := r.catalog(ctx, t.p, false)
	if err != nil {
		return
	}
	for _, m := range ms {
		if m.ID == t.model {
			if usd, ok := estimate(m.Prices, *u, x); ok {
				u.CostUSD, u.CostEstimated = &usd, true
			}
			return
		}
	}
}

var (
	perMillion  = regexp.MustCompile(`\b(1m|m|million)\b`)
	perThousand = regexp.MustCompile(`\b(1k|k|thousand)\b`)
)

// estimate works out a cost from listed prices. ok is false when no price applied to anything the call used.
func estimate(prices []llmc.Price, u llmc.Usage, x used) (usd float64, ok bool) {
	var input, cachedInput, output float64
	var haveInput, haveCached, haveOutput bool
	add := func(price float64, n float64) {
		if n > 0 && price > 0 {
			usd += price * n
			ok = true
		}
	}
	for _, p := range prices {
		unit := strings.ToLower(p.Unit)
		price := p.USD
		switch {
		case perMillion.MatchString(unit):
			price /= 1e6
		case perThousand.MatchString(unit):
			price /= 1e3
		}
		switch {
		// what a unit counts comes first: "completion per second" is seconds, not output tokens
		case strings.Contains(unit, "second"):
			add(price, u.Seconds)
		case strings.Contains(unit, "minute"):
			add(price, u.Seconds/60)
		case strings.Contains(unit, "character") || strings.Contains(unit, "char"):
			add(price, float64(x.chars))
		case strings.Contains(unit, "request"):
			add(price, 1)
		case (strings.Contains(unit, "image") || strings.Contains(unit, "picture")) && !strings.Contains(unit, "input") && !strings.Contains(unit, "token"):
			add(price, float64(x.files))
		case strings.Contains(unit, "cache") && (strings.Contains(unit, "input") || strings.Contains(unit, "prompt") || strings.Contains(unit, "read")):
			cachedInput, haveCached = price, true
		case strings.Contains(unit, "input") || strings.Contains(unit, "prompt"):
			if strings.Contains(unit, "image") || strings.Contains(unit, "audio") {
				continue // prices for media sent in: not counted
			}
			input, haveInput = price, true
		case strings.Contains(unit, "output") || strings.Contains(unit, "completion"):
			output, haveOutput = price, true
		}
	}
	in, cached := float64(u.InputTokens), float64(u.CachedTokens)
	if cached > in {
		cached = in
	}
	if haveInput {
		add(input, in-cached)
		if !haveCached {
			add(input, cached)
		}
	}
	if haveCached {
		add(cachedInput, cached)
	}
	if haveOutput {
		add(output, float64(u.OutputTokens))
	}
	return usd, ok
}
