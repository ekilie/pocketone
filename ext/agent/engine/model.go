// Package engine is a pure-Go, CPU-only inference engine for small Qwen3
// models stored as GGUF files (Q4_K, Q6_K, Q8_0, F16 and F32 tensors).
package engine

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
)

// LoadOptions configures Load.
type LoadOptions struct {
	Threads int // worker threads for matmuls; 0 = number of performance cores (GOMAXPROCS, capped at 8)
}

// Info describes a loaded model.
type Info struct {
	Arch          string
	Name          string
	Params        int64
	ContextLength int
	Quant         string // file type, e.g. Q4_K_M
	TensorTypes   map[string]int
	FileSize      int64
	Layers        int
	Embedding     int
	Threads       int
	Kernels       string
}

type weight struct {
	typ      uint32
	cols     int // row length (input dim)
	rows     int // output dim
	rowBytes int
	data     []byte
}

func (w *weight) row(r int) []byte { return w.data[r*w.rowBytes : (r+1)*w.rowBytes] }

type layer struct {
	attnNorm, ffnNorm []float32
	qNorm, kNorm      []float32
	wq, wk, wv, wo    *weight
	wgate, wup, wdown *weight
}

type hparams struct {
	dim, layers, heads, kvHeads, headDim, ffn, vocab, ctx int
	eps, ropeBase                                         float64
}

// Model is a loaded GGUF model. Weights are mmapped and shared read-only, so
// several Generate calls may run concurrently on one Model.
type Model struct {
	g      *ggufFile
	hp     hparams
	embd   *weight
	out    *weight
	norm   []float32
	layers []layer
	tok    *tokenizer
	pool   *pool
	info   Info
	invFrq []float64

	stateMu sync.Mutex
	states  []*state // idle generation states (KV caches), reused to avoid GC churn
}

// getState returns an idle state with room for maxCtx positions and batches of maxN.
func (m *Model) getState(maxCtx, maxN int) *state {
	m.stateMu.Lock()
	for i, s := range m.states {
		if s.maxCtx >= maxCtx && s.maxN >= maxN {
			m.states = append(m.states[:i], m.states[i+1:]...)
			m.stateMu.Unlock()
			return s
		}
	}
	m.stateMu.Unlock()
	// round up so similar requests can share states
	maxCtx = (maxCtx + 255) &^ 255
	maxCtx = min(maxCtx, m.hp.ctx)
	return m.newState(maxCtx, maxN)
}

func (m *Model) putState(s *state) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	if len(m.states) < 4 {
		m.states = append(m.states, s)
	}
}

var fileTypeNames = map[int]string{
	0: "F32", 1: "F16", 2: "Q4_0", 3: "Q4_1", 7: "Q8_0", 8: "Q5_0", 9: "Q5_1", 10: "Q2_K",
	11: "Q3_K_S", 12: "Q3_K_M", 13: "Q3_K_L", 14: "Q4_K_S", 15: "Q4_K_M", 16: "Q5_K_S",
	17: "Q5_K_M", 18: "Q6_K", 32: "BF16",
}

// Load opens a GGUF model file.
func Load(path string, opts LoadOptions) (*Model, error) {
	g, err := openGGUF(path)
	if err != nil {
		return nil, err
	}
	m, err := newModel(g, opts)
	if err != nil {
		g.close()
		return nil, err
	}
	return m, nil
}

func newModel(g *ggufFile, opts LoadOptions) (*Model, error) {
	arch := g.getString("general.architecture")
	if arch != "qwen3" {
		return nil, fmt.Errorf("engine: unsupported architecture %q (want qwen3)", arch)
	}
	m := &Model{g: g}
	req := func(k string) (int, error) {
		v, ok := g.getInt(arch + "." + k)
		if !ok {
			return 0, fmt.Errorf("engine: missing metadata %s.%s", arch, k)
		}
		return v, nil
	}
	hp := &m.hp
	var err error
	for _, kv := range []struct {
		k string
		p *int
	}{
		{"embedding_length", &hp.dim}, {"block_count", &hp.layers}, {"attention.head_count", &hp.heads},
		{"feed_forward_length", &hp.ffn}, {"context_length", &hp.ctx},
	} {
		if *kv.p, err = req(kv.k); err != nil {
			return nil, err
		}
	}
	hp.kvHeads = hp.heads
	if v, ok := g.getInt(arch + ".attention.head_count_kv"); ok {
		hp.kvHeads = v
	}
	hp.headDim = hp.dim / hp.heads
	if v, ok := g.getInt(arch + ".attention.key_length"); ok {
		hp.headDim = v
	}
	if v, ok := g.getInt(arch + ".attention.value_length"); ok && v != hp.headDim {
		return nil, errors.New("engine: key_length != value_length not supported")
	}
	hp.eps = 1e-6
	if v, ok := g.getFloat(arch + ".attention.layer_norm_rms_epsilon"); ok {
		hp.eps = v
	}
	hp.ropeBase = 10000
	if v, ok := g.getFloat(arch + ".rope.freq_base"); ok {
		hp.ropeBase = v
	}
	if hp.heads%hp.kvHeads != 0 || hp.headDim%2 != 0 {
		return nil, errors.New("engine: bad head configuration")
	}

	getW := func(name string, cols, rows int) (*weight, error) {
		ti, ok := g.Tensors[name]
		if !ok {
			return nil, fmt.Errorf("engine: missing tensor %s", name)
		}
		if ti.Data == nil {
			return nil, fmt.Errorf("engine: tensor %s has unsupported type %s", name, typeName(ti.Type))
		}
		if len(ti.Dims) != 2 || int(ti.Dims[0]) != cols || int(ti.Dims[1]) != rows {
			return nil, fmt.Errorf("engine: tensor %s has shape %v, want [%d %d]", name, ti.Dims, cols, rows)
		}
		switch ti.Type {
		case typeQ4_K, typeQ6_K:
			if cols%qk != 0 {
				return nil, fmt.Errorf("engine: %s row length %d not a multiple of 256", name, cols)
			}
		case typeQ8_0:
			if cols%32 != 0 {
				return nil, fmt.Errorf("engine: %s row length %d not a multiple of 32", name, cols)
			}
		case typeF16, typeF32:
		default:
			return nil, fmt.Errorf("engine: tensor %s: type %s not implemented", name, typeName(ti.Type))
		}
		return &weight{typ: ti.Type, cols: cols, rows: rows, rowBytes: len(ti.Data) / rows, data: ti.Data}, nil
	}
	getV := func(name string, n int) ([]float32, error) {
		ti, ok := g.Tensors[name]
		if !ok {
			return nil, fmt.Errorf("engine: missing tensor %s", name)
		}
		if ti.Type != typeF32 && ti.Type != typeF16 {
			return nil, fmt.Errorf("engine: tensor %s: want F32, got %s", name, typeName(ti.Type))
		}
		if int(ti.Dims[0]) != n {
			return nil, fmt.Errorf("engine: tensor %s has length %d, want %d", name, ti.Dims[0], n)
		}
		out := make([]float32, n)
		dequantRow(ti.Type, ti.Data, out)
		return out, nil
	}

	te, ok := g.Tensors["token_embd.weight"]
	if !ok || len(te.Dims) != 2 {
		return nil, errors.New("engine: missing token_embd.weight")
	}
	hp.vocab = int(te.Dims[1])
	if m.embd, err = getW("token_embd.weight", hp.dim, hp.vocab); err != nil {
		return nil, err
	}
	if _, ok := g.Tensors["output.weight"]; ok {
		if m.out, err = getW("output.weight", hp.dim, hp.vocab); err != nil {
			return nil, err
		}
	} else {
		m.out = m.embd
	}
	if m.norm, err = getV("output_norm.weight", hp.dim); err != nil {
		return nil, err
	}
	qd, kvd := hp.heads*hp.headDim, hp.kvHeads*hp.headDim
	m.layers = make([]layer, hp.layers)
	for i := range m.layers {
		l := &m.layers[i]
		p := fmt.Sprintf("blk.%d.", i)
		for _, s := range []struct {
			name       string
			dst        **weight
			cols, rows int
		}{
			{"attn_q", &l.wq, hp.dim, qd}, {"attn_k", &l.wk, hp.dim, kvd}, {"attn_v", &l.wv, hp.dim, kvd},
			{"attn_output", &l.wo, qd, hp.dim}, {"ffn_gate", &l.wgate, hp.dim, hp.ffn},
			{"ffn_up", &l.wup, hp.dim, hp.ffn}, {"ffn_down", &l.wdown, hp.ffn, hp.dim},
		} {
			if *s.dst, err = getW(p+s.name+".weight", s.cols, s.rows); err != nil {
				return nil, err
			}
		}
		for _, s := range []struct {
			name string
			dst  *[]float32
			n    int
		}{
			{"attn_norm", &l.attnNorm, hp.dim}, {"ffn_norm", &l.ffnNorm, hp.dim},
			{"attn_q_norm", &l.qNorm, hp.headDim}, {"attn_k_norm", &l.kNorm, hp.headDim},
		} {
			if *s.dst, err = getV(p+s.name+".weight", s.n); err != nil {
				return nil, err
			}
		}
	}
	if m.tok, err = newTokenizer(g); err != nil {
		return nil, err
	}
	m.invFrq = make([]float64, hp.headDim/2)
	for i := range m.invFrq {
		m.invFrq[i] = math.Pow(hp.ropeBase, -2*float64(i)/float64(hp.headDim))
	}

	threads := opts.Threads
	if threads <= 0 {
		threads = min(runtime.GOMAXPROCS(0), 8)
	}
	m.pool = newPool(threads)

	var params int64
	types := map[string]int{}
	for _, ti := range g.Tensors {
		n := int64(1)
		for _, d := range ti.Dims {
			n *= int64(d)
		}
		params += n
		types[typeName(ti.Type)]++
	}
	ft, _ := g.getInt("general.file_type")
	q := fileTypeNames[ft]
	if q == "" {
		q = fmt.Sprintf("file_type %d", ft)
	}
	m.info = Info{
		Arch: arch, Name: g.getString("general.name"), Params: params, ContextLength: hp.ctx,
		Quant: q, TensorTypes: types, FileSize: g.size, Layers: hp.layers, Embedding: hp.dim,
		Threads: threads, Kernels: kernelName,
	}
	return m, nil
}

// Close releases the worker pool and unmaps the file. No Generate call may be running.
func (m *Model) Close() error {
	if m.pool != nil {
		m.pool.close()
		m.pool = nil
	}
	return m.g.close()
}

// Info returns model metadata.
func (m *Model) Info() Info { return m.info }

// Tokenize converts raw text (special tokens recognized) into token ids.
func (m *Model) Tokenize(s string) []int { return m.tok.encode(s) }

// Detokenize converts token ids back to text.
func (m *Model) Detokenize(ids []int) string {
	var b strings.Builder
	for _, id := range ids {
		b.Write(m.tok.tokenBytes(id))
	}
	return b.String()
}
