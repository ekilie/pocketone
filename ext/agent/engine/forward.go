package engine

import (
	"math"
)

// acts holds n activation rows of length k in the forms the weights need.
type acts struct {
	n, k           int
	f              []float32
	q8k, q80       []byte
	q80x4          []byte // groups of 4 rows interleaved, for the 4-column Q8_0 kernel
	need8k, need80 bool
}

func newActs(maxN, k int) *acts {
	a := &acts{k: k, f: make([]float32, maxN*k)}
	if k%qk == 0 {
		a.q8k = make([]byte, maxN*k/qk*q8kBytes)
	}
	if k%32 == 0 {
		a.q80 = make([]byte, maxN*k/32*q80aBytes)
		if maxN >= 4 {
			a.q80x4 = make([]byte, maxN/4*k/32*q80x4Bytes)
		}
	}
	return a
}

// prepare records which quantized forms the given weights need.
func (a *acts) prepare(ws ...*weight) {
	a.need8k, a.need80 = false, false
	for _, w := range ws {
		switch w.typ {
		case typeQ4_K, typeQ6_K:
			a.need8k = true
		case typeQ8_0:
			a.need80 = true
		}
	}
}

// quantizeRow quantizes elements [e0,e1) of row t (e0, e1 multiples of 256,
// or e1 == k) into the forms prepare asked for.
func (a *acts) quantizeRow(t, e0, e1 int) {
	k := a.k
	x := a.f[t*k+e0 : t*k+e1]
	if a.need8k {
		quantizeQ8K(x, a.q8k[(t*k+e0)/qk*q8kBytes:])
	}
	if a.need80 {
		quantizeQ8_0(x, a.q80[(t*k+e0)/32*q80aBytes:])
		if a.q80x4 != nil && t < a.n&^3 {
			nbr := k / 32
			interleaveQ8_0(a.q80[(t*k+e0)/32*q80aBytes:], a.q80x4[((t/4)*nbr+e0/32)*q80x4Bytes:], t%4, (e1-e0)/32)
		}
	}
}

// quantize prepares the quantized forms needed by the given weights.
func (a *acts) quantize(p *pool, ws ...*weight) {
	a.prepare(ws...)
	for t := 0; t < a.n; t++ {
		a.quantizeRow(t, 0, a.k)
	}
}

// rowDot returns a function computing dot(weight row bytes, activation row t).
func rowDot(w *weight, a *acts) func(row []byte, t int) float32 {
	k := a.k
	if floatRef {
		return func(row []byte, t int) float32 {
			tmp := make([]float32, k)
			dequantRow(w.typ, row, tmp)
			var s float64
			for i := range tmp {
				s += float64(tmp[i]) * float64(a.f[t*k+i])
			}
			return float32(s)
		}
	}
	switch w.typ {
	case typeQ4_K:
		nb, sz := k/qk, k/qk*q8kBytes
		return func(row []byte, t int) float32 { return dotQ4K(row, a.q8k[t*sz:], nb) }
	case typeQ6_K:
		nb, sz := k/qk, k/qk*q8kBytes
		return func(row []byte, t int) float32 { return dotQ6K(row, a.q8k[t*sz:], nb) }
	case typeQ8_0:
		nb, sz := k/32, k/32*q80aBytes
		return func(row []byte, t int) float32 { return dotQ80(row, a.q80[t*sz:], nb) }
	case typeF16:
		return func(row []byte, t int) float32 { return dotF16(row, a.f[t*k:(t+1)*k]) }
	case typeF32:
		return func(row []byte, t int) float32 {
			var s float32
			for i := 0; i < k; i++ {
				s += math.Float32frombits(uint32(row[4*i])|uint32(row[4*i+1])<<8|uint32(row[4*i+2])<<16|uint32(row[4*i+3])<<24) * a.f[t*k+i]
			}
			return s
		}
	}
	panic("rowDot: unsupported type " + typeName(w.typ))
}

// floatRef switches matmuls to dequantized weights times float activations (debugging).
var floatRef = false

// rowDot4 returns a function computing 4 dots (activation rows t..t+3), or nil.
func rowDot4(w *weight, a *acts) func(row []byte, t int, out *[4]float32) {
	if floatRef {
		return nil
	}
	k := a.k
	sz := k / qk * q8kBytes
	switch {
	case w.typ == typeQ4_K && dotQ4Kx4 != nil:
		return func(row []byte, t int, out *[4]float32) { dotQ4Kx4(row, a.q8k[t*sz:], sz, k/qk, out) }
	case w.typ == typeQ6_K && dotQ6Kx4 != nil:
		return func(row []byte, t int, out *[4]float32) { dotQ6Kx4(row, a.q8k[t*sz:], sz, k/qk, out) }
	case w.typ == typeQ8_0 && dotQ80x4 != nil && a.q80x4 != nil:
		nb := k / 32
		return func(row []byte, t int, out *[4]float32) { dotQ80x4(row, a.q80x4[(t/4)*nb*q80x4Bytes:], nb, out) }
	case w.typ == typeF16 && dotF16x4 != nil:
		return func(row []byte, t int, out *[4]float32) { dotF16x4(row, a.f[t*k:], k, out) }
	}
	return nil
}

type mmSeg struct {
	w      *weight
	out    []float32 // out[t*stride + r]
	stride int
}

// matmul computes out = W·a for each segment, for activation rows [t0, a.n),
// writing output row t at index (t-t0). Rows of all segments are split across
// the pool in tiles; each tile walks tokens in the outer loop so the tile's
// weights stay in L1 while activations stream.
func (m *Model) matmul(p *pool, a *acts, t0 int, segs ...mmSeg) {
	type tileSeg struct {
		dot   func([]byte, int) float32
		dot4  func([]byte, int, *[4]float32)
		first int // first tile index
	}
	n := a.n - t0
	tile := 16
	if n == 1 {
		tile = 8
	}
	ts := make([]tileSeg, len(segs))
	total := 0
	for i, s := range segs {
		ts[i] = tileSeg{dot: rowDot(s.w, a), dot4: rowDot4(s.w, a), first: total}
		total += (s.w.rows + tile - 1) / tile
	}
	chunk := 1
	if total > 2048 {
		chunk = 4
	}
	p.parallelFor(total, chunk, func(lo, hi int) {
		for ti := lo; ti < hi; ti++ {
			si := len(ts) - 1
			for si > 0 && ts[si].first > ti {
				si--
			}
			s := segs[si]
			r0 := (ti - ts[si].first) * tile
			r1 := min(r0+tile, s.w.rows)
			dot, dot4 := ts[si].dot, ts[si].dot4
			t := t0
			if dot4 != nil {
				var o4 [4]float32
				for ; t+4 <= a.n; t += 4 {
					o := s.out[(t-t0)*s.stride:]
					for r := r0; r < r1; r++ {
						dot4(s.w.row(r), t, &o4)
						o[r] = o4[0]
						o[s.stride+r] = o4[1]
						o[2*s.stride+r] = o4[2]
						o[3*s.stride+r] = o4[3]
					}
				}
			}
			for ; t < a.n; t++ {
				o := s.out[(t-t0)*s.stride:]
				for r := r0; r < r1; r++ {
					o[r] = dot(s.w.row(r), t)
				}
			}
		}
	})
}

func rmsnorm(out, x, w []float32, eps float64) {
	var ss float64
	for _, v := range x {
		ss += float64(v) * float64(v)
	}
	scale := float32(1 / math.Sqrt(ss/float64(len(x))+eps))
	for i, v := range x {
		out[i] = v * scale * w[i]
	}
}

// state is one generation's KV cache and scratch buffers.
type state struct {
	maxCtx, maxN int
	ctxP         int         // maxCtx rounded up to 16
	kc, vc       [][]float32 // per layer: kc [kvHeads][headDim][ctxP] (transposed), vc [ctxP][kvHeads*headDim]
	qbuf, obuf   [][]float32 // per worker: 4 gathered queries / outputs
	x            []float32   // residual, maxN*dim
	an, aa, af   *acts       // normed input (dim), attention out (q dim), ffn hidden (ffn)
	q, k, v      []float32
	att          []float32
	gate, up     []float32
	tmp          []float32
	logits       []float32
	scores       [][]float32 // per worker
	cos, sin     []float32   // maxN * headDim/2
	lastAct      *acts
}

func (m *Model) newState(maxCtx, maxN int) *state {
	hp := m.hp
	qd, kvd := hp.heads*hp.headDim, hp.kvHeads*hp.headDim
	s := &state{maxCtx: maxCtx, maxN: maxN, ctxP: (maxCtx + 15) &^ 15}
	s.kc = make([][]float32, hp.layers)
	s.vc = make([][]float32, hp.layers)
	for i := range s.kc {
		s.kc[i] = make([]float32, s.ctxP*kvd)
		s.vc[i] = make([]float32, s.ctxP*kvd)
	}
	s.x = make([]float32, maxN*hp.dim)
	s.an = newActs(maxN, hp.dim)
	s.aa = newActs(maxN, qd)
	s.af = newActs(maxN, hp.ffn)
	s.q = make([]float32, maxN*qd)
	s.k = make([]float32, maxN*kvd)
	s.v = make([]float32, maxN*kvd)
	s.gate = make([]float32, maxN*hp.ffn)
	s.up = make([]float32, maxN*hp.ffn)
	s.tmp = make([]float32, maxN*hp.dim)
	s.logits = make([]float32, hp.vocab)
	s.scores = make([][]float32, m.pool.n)
	s.qbuf = make([][]float32, m.pool.n)
	s.obuf = make([][]float32, m.pool.n)
	for i := range s.scores {
		s.scores[i] = make([]float32, 4*s.ctxP)
		s.qbuf[i] = make([]float32, 4*hp.headDim)
		s.obuf[i] = make([]float32, 4*hp.headDim)
	}
	s.cos = make([]float32, maxN*hp.headDim/2)
	s.sin = make([]float32, maxN*hp.headDim/2)
	s.lastAct = newActs(1, hp.dim)
	return s
}

// forward runs tokens (at positions pos..pos+len-1) through the model and
// fills s.logits for the last token. len(tokens) <= s.maxN.
func (m *Model) forward(s *state, tokens []int, pos int) {
	hp := m.hp
	p := m.pool
	n := len(tokens)
	dim, hd := hp.dim, hp.headDim
	qd, kvd := hp.heads*hd, hp.kvHeads*hd
	half := hd / 2

	for t, id := range tokens {
		dequantRow(m.embd.typ, m.embd.row(id), s.x[t*dim:(t+1)*dim])
		for i := 0; i < half; i++ {
			th := float64(pos+t) * m.invFrq[i]
			s.cos[t*half+i] = float32(math.Cos(th))
			s.sin[t*half+i] = float32(math.Sin(th))
		}
	}
	s.an.n, s.aa.n, s.af.n = n, n, n
	group := hp.heads / hp.kvHeads
	scale := float32(1 / math.Sqrt(float64(hd)))

	// perToken runs f for each token, in parallel for batches.
	perToken := func(f func(t int)) {
		if n < 4 {
			for t := 0; t < n; t++ {
				f(t)
			}
			return
		}
		p.parallelFor(n, 1, func(lo, hi int) {
			for t := lo; t < hi; t++ {
				f(t)
			}
		})
	}
	nblk := hp.ffn / qk
	if hp.ffn%qk != 0 {
		nblk = 1
	}

	for li := range m.layers {
		l := &m.layers[li]
		s.an.prepare(l.wq, l.wk, l.wv)
		perToken(func(t int) {
			x := s.x[t*dim : (t+1)*dim]
			if li > 0 { // residual from the previous layer's FFN
				for i, v := range s.tmp[t*dim : (t+1)*dim] {
					x[i] += v
				}
			}
			rmsnorm(s.an.f[t*dim:(t+1)*dim], x, l.attnNorm, hp.eps)
			s.an.quantizeRow(t, 0, dim)
		})
		m.matmul(p, s.an, 0, mmSeg{l.wq, s.q, qd}, mmSeg{l.wk, s.k, kvd}, mmSeg{l.wv, s.v, kvd})

		// per-head QK norm + RoPE (NeoX), then store K/V in the cache
		kc, vc, ctxP := s.kc[li], s.vc[li], s.ctxP
		perToken(func(t int) {
			cs, sn := s.cos[t*half:(t+1)*half], s.sin[t*half:(t+1)*half]
			for h := 0; h < hp.heads; h++ {
				v := s.q[t*qd+h*hd : t*qd+(h+1)*hd]
				rmsnorm(v, v, l.qNorm, hp.eps)
				rope(v, cs, sn)
			}
			for h := 0; h < hp.kvHeads; h++ {
				v := s.k[t*kvd+h*hd : t*kvd+(h+1)*hd]
				rmsnorm(v, v, l.kNorm, hp.eps)
				rope(v, cs, sn)
			}
			for i, v := range s.k[t*kvd : (t+1)*kvd] {
				kc[i*ctxP+pos+t] = v
			}
			copy(vc[(pos+t)*kvd:(pos+t+1)*kvd], s.v[t*kvd:(t+1)*kvd])
		})

		// Causal attention. Queries sharing a KV head are processed four at a
		// time (items (t, head-in-group) in order), so each K/V row loaded
		// serves four dot products.
		att := s.aa.f
		items := n * group
		quads := (items + 3) / 4
		p.parallelForW(hp.kvHeads*quads, 1, func(w, lo, hi int) {
			sc, qb, ob := s.scores[w], s.qbuf[w], s.obuf[w]
			for task := lo; task < hi; task++ {
				kvh, qi := task/quads, task%quads
				var ts, hs [4]int
				for i := 0; i < 4; i++ {
					it := min(4*qi+i, items-1)
					ts[i], hs[i] = it/group, kvh*group+it%group
					copy(qb[i*hd:(i+1)*hd], s.q[ts[i]*qd+hs[i]*hd:ts[i]*qd+(hs[i]+1)*hd])
				}
				ctxQ := pos + ts[3] + 1 // items are in token order: the last has the longest context
				nblk := (ctxQ + 15) / 16
				scores4(qb, kc[kvh*hd*ctxP:], ctxP, nblk, sc, ctxP)
				n4 := (ctxQ + 3) / 4
				for i := 0; i < 4; i++ {
					row := sc[i*ctxP : i*ctxP+4*n4]
					ctx := pos + ts[i] + 1
					mx := float32(math.Inf(-1))
					for j := 0; j < ctx; j++ {
						row[j] *= scale
						mx = max(mx, row[j])
					}
					var sum float32
					for j := 0; j < ctx; j++ {
						e := expf(row[j] - mx)
						row[j] = e
						sum += e
					}
					inv := 1 / sum
					for j := 0; j < ctx; j++ {
						row[j] *= inv
					}
					clear(row[ctx:])
				}
				wsum4(sc, ctxP, vc[kvh*hd:], kvd, n4, ob, hd, hd)
				for i := 0; i < 4; i++ {
					if 4*qi+i < items {
						copy(att[ts[i]*qd+hs[i]*hd:ts[i]*qd+(hs[i]+1)*hd], ob[i*hd:(i+1)*hd])
					}
				}
			}
		})
		s.aa.prepare(l.wo)
		perToken(func(t int) { s.aa.quantizeRow(t, 0, qd) })
		m.matmul(p, s.aa, 0, mmSeg{l.wo, s.tmp, dim})

		s.an.prepare(l.wgate, l.wup)
		perToken(func(t int) {
			x := s.x[t*dim : (t+1)*dim]
			for i, v := range s.tmp[t*dim : (t+1)*dim] {
				x[i] += v
			}
			rmsnorm(s.an.f[t*dim:(t+1)*dim], x, l.ffnNorm, hp.eps)
			s.an.quantizeRow(t, 0, dim)
		})
		m.matmul(p, s.an, 0, mmSeg{l.wgate, s.gate, hp.ffn}, mmSeg{l.wup, s.up, hp.ffn})

		// SwiGLU + quantize, split over (token, 256-block) so decode parallelizes too
		s.af.prepare(l.wdown)
		bs := hp.ffn / nblk
		p.parallelFor(n*nblk, max(1, nblk/8), func(lo, hi int) {
			for task := lo; task < hi; task++ {
				t, b := task/nblk, task%nblk
				o := t*hp.ffn + b*bs
				hf, g, u := s.af.f[o:o+bs], s.gate[o:o+bs], s.up[o:o+bs]
				for i := range hf {
					hf[i] = g[i] / (1 + expf(-g[i])) * u[i]
				}
				s.af.quantizeRow(t, b*bs, (b+1)*bs)
			}
		})
		m.matmul(p, s.af, 0, mmSeg{l.wdown, s.tmp, dim})
	}
	for i := 0; i < n*dim; i++ {
		s.x[i] += s.tmp[i]
	}

	la := s.lastAct
	la.n = 1
	rmsnorm(la.f, s.x[(n-1)*dim:n*dim], m.norm, hp.eps)
	la.quantize(p, m.out)
	m.matmul(p, la, 0, mmSeg{m.out, s.logits, hp.vocab})
}

func rope(v, cs, sn []float32) {
	half := len(cs)
	for i := 0; i < half; i++ {
		x0, x1 := v[i], v[i+half]
		v[i] = x0*cs[i] - x1*sn[i]
		v[i+half] = x0*sn[i] + x1*cs[i]
	}
}
