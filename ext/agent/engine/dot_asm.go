//go:build arm64 || amd64

package engine

import (
	"os"
	"unsafe"
)

// Assembly kernels, implemented in dot_arm64.s (NEON, SDOT) and dot_amd64.s
// (AVX2, FMA, F16C) with identical signatures. Lengths and strides follow the
// comments in the .s files; the wrappers below check bounds and fall back to
// Go for shapes the assembly doesn't take.

//go:noescape
func dotQ4KQ8KAsm(x, y unsafe.Pointer, nb int) float32

//go:noescape
func dotQ6KQ8KAsm(x, y unsafe.Pointer, nb int) float32

//go:noescape
func dotQ8_0Asm(x, y unsafe.Pointer, nb int) float32

//go:noescape
func dotF16F32Asm(x, y unsafe.Pointer, n int) float32

//go:noescape
func dotF32Asm(x, y unsafe.Pointer, n int) float32

//go:noescape
func axpyAsm(a float32, x, y unsafe.Pointer, n int)

//go:noescape
func dotQ4KQ8Kx4Asm(x, y unsafe.Pointer, stride, nb int, out *float32)

//go:noescape
func dotQ6KQ8Kx4Asm(x, y unsafe.Pointer, stride, nb int, out *float32)

//go:noescape
func dotF16F32x4Asm(x, y unsafe.Pointer, stride, n int, out *float32)

//go:noescape
func dotQ8_0x4Asm(x, y unsafe.Pointer, nb int, out *float32)

//go:noescape
func scores4Asm(q, kT unsafe.Pointer, dstride, nblk, hd int, out unsafe.Pointer, ostride int)

//go:noescape
func wsum4Asm(p unsafe.Pointer, pstride int, v unsafe.Pointer, vstride, n4, hd int, out unsafe.Pointer, ostride int)

func init() {
	if os.Getenv("AGENT_ENGINE_NOASM") != "" {
		return
	}
	// asmFeatures (per architecture) reports which kernel families the CPU runs:
	// float covers F16/F32 and attention, ints the 8-bit dot products.
	float, ints, name := asmFeatures()
	if float {
		useFloatAsm()
	}
	if ints {
		useIntAsm()
	}
	if float || ints {
		kernelName = name
	}
}

func useFloatAsm() {
	dotF16 = func(x []byte, y []float32) float32 {
		n := len(y) &^ 15
		s := float32(0)
		if n > 0 {
			_ = x[2*n-1]
			s = dotF16F32Asm(bptr(x), fptr(y), n)
		}
		if n < len(y) {
			s += dotF16F32Go(x[2*n:], y[n:])
		}
		return s
	}
	dotF32 = func(x, y []float32) float32 {
		n := len(x) &^ 15
		s := float32(0)
		if n > 0 {
			_ = y[n-1]
			s = dotF32Asm(fptr(x), fptr(y), n)
		}
		if n < len(x) {
			s += dotF32Go(x[n:], y[n:len(x)])
		}
		return s
	}
	axpy = func(a float32, x, y []float32) {
		n := len(x) &^ 15
		if n > 0 {
			_ = y[len(x)-1]
			axpyAsm(a, fptr(x), fptr(y), n)
		}
		if n < len(x) {
			axpyGo(a, x[n:], y[n:len(x)])
		}
	}
	dotF16x4 = func(x []byte, y []float32, stride int, out *[4]float32) {
		n := len(x) / 2
		if n%16 != 0 {
			for c := 0; c < 4; c++ {
				out[c] = dotF16F32Go(x, y[c*stride:c*stride+n])
			}
			return
		}
		_ = y[3*stride+n-1]
		dotF16F32x4Asm(bptr(x), fptr(y), stride*4, n, &out[0])
	}
	scores4 = func(q, kT []float32, dstride, nblk int, out []float32, ostride int) {
		hd := len(q) / 4
		if hd%4 != 0 || nblk == 0 {
			scores4Go(q, kT, dstride, nblk, out, ostride)
			return
		}
		_, _ = kT[(hd-1)*dstride+16*nblk-1], out[3*ostride+16*nblk-1]
		scores4Asm(fptr(q), fptr(kT), dstride*4, nblk, hd, fptr(out), ostride*4)
	}
	wsum4 = func(p []float32, pstride int, v []float32, vstride, n4 int, out []float32, ostride, hd int) {
		if hd%16 != 0 {
			wsum4Go(p, pstride, v, vstride, n4, out, ostride, hd)
			return
		}
		if n4 > 0 {
			_, _ = p[3*pstride+4*n4-1], v[(4*n4-1)*vstride+hd-1]
		}
		_ = out[3*ostride+hd-1]
		wsum4Asm(fptr(p), pstride*4, fptr(v), vstride*4, n4, hd, fptr(out), ostride*4)
	}
}

func useIntAsm() {
	dotQ4K = func(x, y []byte, nb int) float32 {
		_, _ = x[nb*q4kBytes-1], y[nb*q8kBytes-1]
		return dotQ4KQ8KAsm(bptr(x), bptr(y), nb)
	}
	dotQ6K = func(x, y []byte, nb int) float32 {
		_, _ = x[nb*q6kBytes-1], y[nb*q8kBytes-1]
		return dotQ6KQ8KAsm(bptr(x), bptr(y), nb)
	}
	dotQ80 = func(x, y []byte, nb int) float32 {
		_, _ = x[nb*q80Bytes-1], y[nb*q80aBytes-1]
		return dotQ8_0Asm(bptr(x), bptr(y), nb)
	}
	dotQ4Kx4 = func(x, y []byte, stride, nb int, out *[4]float32) {
		_, _ = x[nb*q4kBytes-1], y[3*stride+nb*q8kBytes-1]
		dotQ4KQ8Kx4Asm(bptr(x), bptr(y), stride, nb, &out[0])
	}
	dotQ6Kx4 = func(x, y []byte, stride, nb int, out *[4]float32) {
		_, _ = x[nb*q6kBytes-1], y[3*stride+nb*q8kBytes-1]
		dotQ6KQ8Kx4Asm(bptr(x), bptr(y), stride, nb, &out[0])
	}
	dotQ80x4 = func(x, y []byte, nb int, out *[4]float32) {
		_, _ = x[nb*q80Bytes-1], y[nb*q80x4Bytes-1]
		dotQ8_0x4Asm(bptr(x), bptr(y), nb, &out[0])
	}
}
