package engine

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// GenerateOptions controls sampling.
type GenerateOptions struct {
	MaxTokens   int     // tokens to generate (default 256)
	Temperature float64 // 0 = greedy
	TopP        float64 // 0 or 1 = off
	TopK        int     // 0 = off
	Seed        uint64
	Stop        []string // stop strings (not included in the output)
	IgnoreEOS   bool     // keep generating past end-of-turn tokens (benchmarks)
}

// Result reports what Generate produced.
type Result struct {
	Text         string
	Tokens       []int
	PromptTokens int
	OutputTokens int
	Prefill      time.Duration // prompt processing, up to the first sampled token
	Decode       time.Duration // remaining tokens
	StopReason   string        // "eos", "stop", "length", "callback", "canceled"
}

// Message is one chat turn.
type Message struct {
	Role    string // system, user, assistant
	Content string
}

// ChatPrompt applies Qwen3's ChatML template with thinking turned off.
func ChatPrompt(messages []Message) string {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString("<|im_start|>")
		b.WriteString(m.Role)
		b.WriteString("\n")
		b.WriteString(m.Content)
		b.WriteString("<|im_end|>\n")
	}
	b.WriteString("<|im_start|>assistant\n<think>\n\n</think>\n\n")
	return b.String()
}

// ChatPrompt is the method form of the package function.
func (m *Model) ChatPrompt(messages []Message) string { return ChatPrompt(messages) }

const prefillBatch = 512

// Generate runs the prompt (raw text; the caller applies the chat template)
// and streams generated text pieces to onToken (which may be nil). Returning
// false from onToken stops generation.
func (m *Model) Generate(ctx context.Context, prompt string, o GenerateOptions, onToken func(piece string) bool) (Result, error) {
	ids := m.Tokenize(prompt)
	return m.GenerateTokens(ctx, ids, o, onToken)
}

// GenerateTokens is Generate on pre-tokenized input.
func (m *Model) GenerateTokens(ctx context.Context, ids []int, o GenerateOptions, onToken func(piece string) bool) (Result, error) {
	res := Result{PromptTokens: len(ids)}
	if len(ids) == 0 {
		return res, errors.New("engine: empty prompt")
	}
	if o.MaxTokens <= 0 {
		o.MaxTokens = 256
	}
	maxCtx := len(ids) + o.MaxTokens
	if maxCtx > m.hp.ctx {
		return res, errors.New("engine: prompt plus max tokens exceeds the model's context length")
	}
	s := m.getState(maxCtx, min(len(ids), prefillBatch))
	defer m.putState(s)
	rng := rand.New(rand.NewPCG(o.Seed, o.Seed^0x9e3779b97f4a7c15))

	start := time.Now()
	for pos := 0; pos < len(ids); pos += prefillBatch {
		if err := ctx.Err(); err != nil {
			res.StopReason = "canceled"
			return res, err
		}
		m.forward(s, ids[pos:min(pos+prefillBatch, len(ids))], pos)
	}

	var text strings.Builder
	var pending []byte // bytes not yet forming complete UTF-8
	emitted := 0       // bytes of text already passed to onToken
	maxStop := 0
	for _, st := range o.Stop {
		maxStop = max(maxStop, len(st))
	}
	pos := len(ids)
	var decodeStart time.Time
	finish := func(reason string) {
		res.StopReason = reason
		res.Text = text.String()
		if !decodeStart.IsZero() {
			res.Decode = time.Since(decodeStart)
		}
	}
	for {
		tok := m.sample(s.logits, o, rng)
		if res.OutputTokens == 0 {
			res.Prefill = time.Since(start)
			decodeStart = time.Now()
		}
		if m.tok.eos[tok] && !o.IgnoreEOS {
			finish("eos")
			break
		}
		res.Tokens = append(res.Tokens, tok)
		res.OutputTokens++
		pending = append(pending, m.tok.tokenBytes(tok)...)
		// move the complete UTF-8 prefix into text
		k := 0
		for k < len(pending) {
			r, size := utf8.DecodeRune(pending[k:])
			if r == utf8.RuneError && size <= 1 && !utf8.FullRune(pending[k:]) {
				break
			}
			k += size
		}
		text.Write(pending[:k])
		pending = pending[k:]

		stopped := false
		full := text.String()
		if maxStop > 0 {
			for _, st := range o.Stop {
				if st == "" {
					continue
				}
				if i := strings.Index(full[max(0, emitted-len(st)):], st); i >= 0 {
					cut := max(0, emitted-len(st)) + i
					text.Reset()
					text.WriteString(full[:cut])
					full = full[:cut]
					stopped = true
					break
				}
			}
		}
		// hold back text that could be the start of a stop string
		safe := len(full)
		if !stopped && maxStop > 0 {
			safe = max(emitted, len(full)-(maxStop-1))
		}
		if safe > emitted && onToken != nil {
			if !onToken(full[emitted:safe]) {
				emitted = safe
				finish("callback")
				return res, nil
			}
		}
		emitted = max(emitted, safe)
		if stopped {
			finish("stop")
			return res, nil
		}
		if res.OutputTokens >= o.MaxTokens {
			if onToken != nil && emitted < len(full) {
				onToken(full[emitted:])
			}
			finish("length")
			return res, nil
		}
		if err := ctx.Err(); err != nil {
			finish("canceled")
			return res, err
		}
		m.forward(s, []int{tok}, pos)
		pos++
	}
	if onToken != nil {
		full := text.String()
		if emitted < len(full) {
			onToken(full[emitted:])
		}
	}
	return res, nil
}

func (m *Model) sample(logits []float32, o GenerateOptions, rng *rand.Rand) int {
	if o.Temperature <= 0 || o.TopK == 1 {
		best, bv := 0, float32(math.Inf(-1))
		for i, v := range logits {
			if v > bv {
				best, bv = i, v
			}
		}
		return best
	}
	type cand struct {
		id int
		p  float32
	}
	k := o.TopK
	if k <= 0 || k > len(logits) {
		k = len(logits)
	}
	// Candidates more than 30/temperature below the best have probability
	// < e^-30 relative to it; dropping them avoids sorting the whole vocabulary.
	mxl := float32(math.Inf(-1))
	for _, v := range logits {
		mxl = max(mxl, v)
	}
	cut := mxl - float32(30*o.Temperature)
	cs := make([]cand, 0, 256)
	for i, v := range logits {
		if v >= cut {
			cs = append(cs, cand{i, v})
		}
	}
	k = min(k, len(cs))
	sort.Slice(cs, func(a, b int) bool { return cs[a].p > cs[b].p })
	cs = cs[:k]
	mx := cs[0].p
	var sum float64
	for i := range cs {
		e := math.Exp(float64(cs[i].p-mx) / o.Temperature)
		cs[i].p = float32(e)
		sum += e
	}
	for i := range cs {
		cs[i].p = float32(float64(cs[i].p) / sum)
	}
	if o.TopP > 0 && o.TopP < 1 {
		var c float64
		for i := range cs {
			c += float64(cs[i].p)
			if c >= o.TopP {
				cs = cs[:i+1]
				break
			}
		}
		sum = c
	} else {
		sum = 1
	}
	r := rng.Float64() * sum
	var c float64
	for _, x := range cs {
		c += float64(x.p)
		if r < c {
			return x.id
		}
	}
	return cs[len(cs)-1].id
}

// TeacherForce runs prompt then cont through the model and, for each
// position of cont, returns the model's greedy choice and the log-probability
// it gives cont's token there (for comparing against another engine).
func (m *Model) TeacherForce(prompt, cont []int) (argmax []int, logprob []float64) {
	s := m.newState(len(prompt)+len(cont)+1, min(len(prompt), prefillBatch))
	for pos := 0; pos < len(prompt); pos += prefillBatch {
		m.forward(s, prompt[pos:min(pos+prefillBatch, len(prompt))], pos)
	}
	for i, tok := range cont {
		best, mx := 0, float32(math.Inf(-1))
		for j, v := range s.logits {
			if v > mx {
				best, mx = j, v
			}
		}
		var sum float64
		for _, v := range s.logits {
			sum += math.Exp(float64(v - mx))
		}
		argmax = append(argmax, best)
		logprob = append(logprob, float64(s.logits[tok]-mx)-math.Log(sum))
		m.forward(s, []int{tok}, len(prompt)+i)
	}
	return argmax, logprob
}
