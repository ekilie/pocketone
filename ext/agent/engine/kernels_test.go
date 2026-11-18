package engine

import (
	"encoding/binary"
	"math"
	"math/rand/v2"
	"testing"
)

func randBlocks(r *rand.Rand, nb, size int, f16off []int) []byte {
	b := make([]byte, nb*size)
	for i := range b {
		b[i] = byte(r.IntN(256))
	}
	// keep f16 scales sane (small positive/negative normals)
	for i := 0; i < nb; i++ {
		for _, o := range f16off {
			h := uint16(0x2000 + r.IntN(0x1000))
			if r.IntN(2) == 0 {
				h |= 0x8000
			}
			binary.LittleEndian.PutUint16(b[i*size+o:], h)
		}
	}
	return b
}

func close32(a, b float32) bool {
	return math.Abs(float64(a-b)) <= 1e-3*math.Max(1, math.Abs(float64(b)))
}

func TestKernels(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	t.Logf("kernels: %s", kernelName)
	for _, nb := range []int{1, 4, 12} {
		x := make([]float32, nb*qk)
		for i := range x {
			x[i] = float32(r.NormFloat64())
		}
		y := make([]byte, nb*q8kBytes)
		quantizeQ8K(x, y)
		w4 := randBlocks(r, nb, q4kBytes, []int{0, 2})
		if a, b := dotQ4K(w4, y, nb), dotQ4KQ8KGo(w4, y, nb); !close32(a, b) {
			t.Errorf("q4k nb=%d: %v vs %v", nb, a, b)
		}
		w6 := randBlocks(r, nb, q6kBytes, []int{208})
		if dotQ4Kx4 != nil {
			xs := make([]float32, 4*nb*qk)
			for i := range xs {
				xs[i] = float32(r.NormFloat64())
			}
			ys := make([]byte, 4*nb*q8kBytes)
			quantizeQ8K(xs, ys)
			var o4, o6 [4]float32
			dotQ4Kx4(w4, ys, nb*q8kBytes, nb, &o4)
			dotQ6Kx4(w6, ys, nb*q8kBytes, nb, &o6)
			for c := 0; c < 4; c++ {
				if b := dotQ4KQ8KGo(w4, ys[c*nb*q8kBytes:], nb); !close32(o4[c], b) {
					t.Errorf("q4k x4 nb=%d col %d: %v vs %v", nb, c, o4[c], b)
				}
				if b := dotQ6KQ8KGo(w6, ys[c*nb*q8kBytes:], nb); !close32(o6[c], b) {
					t.Errorf("q6k x4 nb=%d col %d: %v vs %v", nb, c, o6[c], b)
				}
			}
		}
		if a, b := dotQ6K(w6, y, nb), dotQ6KQ8KGo(w6, y, nb); !close32(a, b) {
			t.Errorf("q6k nb=%d: %v vs %v", nb, a, b)
		}
		// check Go dot against dequantized float dot, with x replaced by its exact Q8_K value
		xq := make([]float32, nb*qk)
		for b := 0; b < nb; b++ {
			d := math.Float32frombits(binary.LittleEndian.Uint32(y[b*q8kBytes:]))
			for j := 0; j < qk; j++ {
				xq[b*qk+j] = d * float32(int8(y[b*q8kBytes+4+j]))
			}
		}
		x0 := x
		x = xq
		deq := make([]float32, nb*qk)
		dequantRow(typeQ4_K, w4, deq)
		ref := dotF32Go(deq, x)
		if a := dotQ4KQ8KGo(w4, y, nb); math.Abs(float64(a-ref)) > 1e-3*math.Max(1, math.Abs(float64(ref))) {
			t.Errorf("q4k vs float nb=%d: %v vs %v", nb, a, ref)
		}
		dequantRow(typeQ6_K, w6, deq)
		ref = dotF32Go(deq, x)
		if a := dotQ6KQ8KGo(w6, y, nb); math.Abs(float64(a-ref)) > 1e-3*math.Max(1, math.Abs(float64(ref))) {
			t.Errorf("q6k vs float nb=%d: %v vs %v", nb, a, ref)
		}
		x = x0
		nb0 := nb * 8
		w8 := randBlocks(r, nb0, q80Bytes, []int{0})
		y8 := make([]byte, nb0*q80aBytes)
		quantizeQ8_0(x, y8)
		if a, b := dotQ80(w8, y8, nb0), dotQ8_0Go(w8, y8, nb0); !close32(a, b) {
			t.Errorf("q8_0 nb=%d: %v vs %v", nb0, a, b)
		}
		if dotQ80x4 != nil {
			xs := make([]float32, 4*nb0*32)
			for i := range xs {
				xs[i] = float32(r.NormFloat64())
			}
			ys := make([]byte, 4*nb0*q80aBytes)
			quantizeQ8_0(xs, ys)
			il := make([]byte, nb0*q80x4Bytes)
			for c := 0; c < 4; c++ {
				interleaveQ8_0(ys[c*nb0*q80aBytes:], il, c, nb0)
			}
			var o [4]float32
			dotQ80x4(w8, il, nb0, &o)
			for c := 0; c < 4; c++ {
				if b := dotQ8_0Go(w8, ys[c*nb0*q80aBytes:], nb0); !close32(o[c], b) {
					t.Errorf("q8_0 x4 col %d: %v vs %v", c, o[c], b)
				}
			}
			// odd block count for the unrolled single kernel
			if a, b := dotQ80(w8, y8, nb0-1), dotQ8_0Go(w8, y8, nb0-1); !close32(a, b) {
				t.Errorf("q8_0 odd nb: %v vs %v", a, b)
			}
		}
		dequantRow(typeQ8_0, w8, deq)
		for b := 0; b < nb0; b++ {
			d := math.Float32frombits(binary.LittleEndian.Uint32(y8[b*q80aBytes:]))
			for j := 0; j < 32; j++ {
				xq[b*32+j] = d * float32(int8(y8[b*q80aBytes+4+j]))
			}
		}
		ref = dotF32Go(deq, xq)
		if a := dotQ8_0Go(w8, y8, nb0); math.Abs(float64(a-ref)) > 1e-3*math.Max(1, math.Abs(float64(ref))) {
			t.Errorf("q8_0 vs float nb=%d: %v vs %v", nb, a, ref)
		}
		// f16 / f32 / axpy
		n := nb*qk + 5
		if nb == 4 {
			n = nb * qk
		}
		hx := make([]byte, 2*n)
		fx := make([]float32, n)
		fy := make([]float32, n)
		for i := 0; i < n; i++ {
			h := uint16(0x3000 + r.IntN(0x1000))
			binary.LittleEndian.PutUint16(hx[2*i:], h)
			fx[i] = float32(r.NormFloat64())
			fy[i] = float32(r.NormFloat64())
		}
		if dotF16x4 != nil && n%16 == 0 {
			y4 := make([]float32, 4*n)
			for i := range y4 {
				y4[i] = float32(r.NormFloat64())
			}
			var o [4]float32
			dotF16x4(hx, y4, n, &o)
			for c := 0; c < 4; c++ {
				if b := dotF16F32Go(hx, y4[c*n:(c+1)*n]); !close32(o[c], b) {
					t.Errorf("f16x4 col %d: %v vs %v", c, o[c], b)
				}
			}
		}
		if a, b := dotF16(hx, fy), dotF16F32Go(hx, fy); !close32(a, b) {
			t.Errorf("f16: %v vs %v", a, b)
		}
		if a, b := dotF32(fx, fy), dotF32Go(fx, fy); !close32(a, b) {
			t.Errorf("f32: %v vs %v", a, b)
		}
		y1 := append([]float32(nil), fy...)
		y2 := append([]float32(nil), fy...)
		axpy(0.5, fx, y1)
		axpyGo(0.5, fx, y2)
		for i := range y1 {
			if !close32(y1[i], y2[i]) {
				t.Fatalf("axpy %d: %v vs %v", i, y1[i], y2[i])
			}
		}
	}
}

func TestAttnKernels(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	hd, stride, n := 128, 1024, 37
	q := make([]float32, hd)
	k := make([]float32, n*stride)
	for i := range q {
		q[i] = float32(r.NormFloat64())
	}
	for i := range k {
		k[i] = float32(r.NormFloat64())
	}
	o1, o2 := make([]float32, n), make([]float32, n)
	dots(q, k[256:], stride, n, o1)
	dotsGo(q, k[256:], stride, n, o2)
	for j := range o1 {
		if !close32(o1[j], o2[j]) {
			t.Fatalf("dots %d: %v vs %v", j, o1[j], o2[j])
		}
	}
	w1, w2 := make([]float32, hd), make([]float32, hd)
	wsum(o2, k[128:], stride, w1)
	for j := range o2 {
		axpyGo(o2[j], k[128+j*stride:128+j*stride+hd], w2)
	}
	for j := range w1 {
		if !close32(w1[j], w2[j]) {
			t.Fatalf("wsum %d: %v vs %v", j, w1[j], w2[j])
		}
	}
	for x := float32(-86.9); x < 88; x += 0.137 {
		a, b := expf(x), float32(math.Exp(float64(x)))
		if math.Abs(float64(a-b)) > 1e-6*float64(b)+1e-38 {
			t.Fatalf("expf(%v) = %v want %v", x, a, b)
		}
	}
}

func TestAttn4Kernels(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))
	hd, P := 128, 80
	q := make([]float32, 4*hd)
	kT := make([]float32, hd*P)
	for i := range q {
		q[i] = float32(r.NormFloat64())
	}
	for i := range kT {
		kT[i] = float32(r.NormFloat64())
	}
	o1, o2 := make([]float32, 4*96), make([]float32, 4*96)
	scores4(q, kT, P, 5, o1, 96)
	scores4Go(q, kT, P, 5, o2, 96)
	for i := range o1 {
		if !close32(o1[i], o2[i]) {
			t.Fatalf("scores4 %d: %v vs %v", i, o1[i], o2[i])
		}
	}
	vs := 1024
	v := make([]float32, 64*vs)
	for i := range v {
		v[i] = float32(r.NormFloat64())
	}
	w1, w2 := make([]float32, 4*200), make([]float32, 4*200)
	wsum4(o2, 96, v[128:], vs, 15, w1, 200, hd)
	wsum4Go(o2, 96, v[128:], vs, 15, w2, 200, hd)
	for i := range w1 {
		if !close32(w1[i], w2[i]) {
			t.Fatalf("wsum4 %d: %v vs %v", i, w1[i], w2[i])
		}
	}
}
