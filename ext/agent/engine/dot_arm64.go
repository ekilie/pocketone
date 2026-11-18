//go:build arm64

package engine

import (
	"encoding/binary"
	"os"
	"runtime"
	"unsafe"
)

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

// hasDotProd reports whether the CPU has the ARMv8.2 SDOT instruction.
func hasDotProd() bool {
	switch runtime.GOOS {
	case "darwin", "ios":
		return true // every Apple Silicon core has it
	case "linux", "android":
		b, err := os.ReadFile("/proc/self/auxv")
		if err != nil {
			return false
		}
		for i := 0; i+16 <= len(b); i += 16 {
			if binary.LittleEndian.Uint64(b[i:]) == 16 { // AT_HWCAP
				return binary.LittleEndian.Uint64(b[i+8:])&(1<<20) != 0 // HWCAP_ASIMDDP
			}
		}
	}
	return false
}

func init() {
	if os.Getenv("AGENT_ENGINE_NOASM") != "" {
		return
	}
	// Float kernels need only base NEON (always present on arm64).
	dotF16 = func(x []byte, y []float32) float32 {
		n := len(y) &^ 15
		s := float32(0)
		if n > 0 {
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
	kernelName = "neon"
	if !hasDotProd() {
		return
	}
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
	kernelName = "neon+dotprod"
}

//go:noescape
func dotQ4KQ8Kx4Asm(x, y unsafe.Pointer, stride, nb int, out *float32)

//go:noescape
func dotQ6KQ8Kx4Asm(x, y unsafe.Pointer, stride, nb int, out *float32)

func init() {
	if kernelName != "neon+dotprod" {
		return
	}
	dotQ4Kx4 = func(x, y []byte, stride, nb int, out *[4]float32) {
		_, _ = x[nb*q4kBytes-1], y[3*stride+nb*q8kBytes-1]
		dotQ4KQ8Kx4Asm(bptr(x), bptr(y), stride, nb, &out[0])
	}
	dotQ6Kx4 = func(x, y []byte, stride, nb int, out *[4]float32) {
		_, _ = x[nb*q6kBytes-1], y[3*stride+nb*q8kBytes-1]
		dotQ6KQ8Kx4Asm(bptr(x), bptr(y), stride, nb, &out[0])
	}
}

//go:noescape
func dotsF32Asm(q, k unsafe.Pointer, kstride, n, hd int, out unsafe.Pointer)

//go:noescape
func wsumF32Asm(p, v unsafe.Pointer, vstride, n, hd int, out unsafe.Pointer)

func init() {
	if os.Getenv("AGENT_ENGINE_NOASM") != "" {
		return
	}
	dots = func(q, k []float32, kstride, n int, out []float32) {
		hd := len(q)
		if hd%16 != 0 || n == 0 {
			dotsGo(q, k, kstride, n, out)
			return
		}
		_, _ = k[(n-1)*kstride+hd-1], out[n-1]
		dotsF32Asm(fptr(q), fptr(k), kstride*4, n, hd, fptr(out))
	}
	wsum = func(p, v []float32, vstride int, out []float32) {
		hd, n := len(out), len(p)
		if hd%32 != 0 || n == 0 {
			wsumGo(p, v, vstride, out)
			return
		}
		_ = v[(n-1)*vstride+hd-1]
		wsumF32Asm(fptr(p), fptr(v), vstride*4, n, hd, fptr(out))
	}
}

//go:noescape
func dotF16F32x4Asm(x, y unsafe.Pointer, stride, n int, out *float32)

func init() {
	if os.Getenv("AGENT_ENGINE_NOASM") != "" {
		return
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
}

//go:noescape
func dotQ8_0x4Asm(x, y unsafe.Pointer, nb int, out *float32)

func init() {
	if kernelName != "neon+dotprod" {
		return
	}
	dotQ80x4 = func(x, y []byte, nb int, out *[4]float32) {
		_, _ = x[nb*q80Bytes-1], y[nb*q80x4Bytes-1]
		dotQ8_0x4Asm(bptr(x), bptr(y), nb, &out[0])
	}
}

//go:noescape
func scores4Asm(q, kT unsafe.Pointer, dstride, nblk, hd int, out unsafe.Pointer, ostride int)

//go:noescape
func wsum4Asm(p unsafe.Pointer, pstride int, v unsafe.Pointer, vstride, n4, hd int, out unsafe.Pointer, ostride int)

func init() {
	if os.Getenv("AGENT_ENGINE_NOASM") != "" {
		return
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
