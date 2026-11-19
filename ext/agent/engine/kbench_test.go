package engine

import (
	"math/rand/v2"
	"testing"
)

func benchKernel(b *testing.B, rows, cols int, size int, dot func(x, y []byte, nb int) float32, wBytesPerBlock int) {
	r := rand.New(rand.NewPCG(1, 2))
	nb := cols / qk
	w := randBlocks(r, rows*nb, wBytesPerBlock, []int{0})
	x := make([]float32, cols)
	for i := range x {
		x[i] = float32(r.NormFloat64())
	}
	y := make([]byte, nb*q8kBytes)
	quantizeQ8K(x, y)
	rb := nb * wBytesPerBlock
	b.SetBytes(int64(len(w)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < rows; j++ {
			dot(w[j*rb:], y, nb)
		}
	}
}

func BenchmarkQ4KL2(b *testing.B)   { benchKernel(b, 2048, 1024, 0, dotQ4K, q4kBytes) }
func BenchmarkQ6KL2(b *testing.B)   { benchKernel(b, 2048, 1024, 0, dotQ6K, q6kBytes) }
func BenchmarkQ4KDRAM(b *testing.B) { benchKernel(b, 151936, 1024, 0, dotQ4K, q4kBytes) }

func benchX4(b *testing.B, x4 func(x, y []byte, stride, nb int, out *[4]float32), wb int) {
	r := rand.New(rand.NewPCG(1, 2))
	rows, cols := 2048, 1024
	nb := cols / qk
	w := randBlocks(r, rows*nb, wb, []int{0})
	x := make([]float32, 4*cols)
	for i := range x {
		x[i] = float32(r.NormFloat64())
	}
	y := make([]byte, 4*nb*q8kBytes)
	quantizeQ8K(x, y)
	rb := nb * wb
	var o [4]float32
	b.SetBytes(int64(len(w)) * 4) // "column-bytes": comparable with the single kernel
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < rows; j++ {
			x4(w[j*rb:], y, nb*q8kBytes, nb, &o)
		}
	}
}

func BenchmarkQ4Kx4L2(b *testing.B) { benchX4(b, dotQ4Kx4, q4kBytes) }
func BenchmarkQ6Kx4L2(b *testing.B) { benchX4(b, dotQ6Kx4, q6kBytes) }

func BenchmarkQ80L2(b *testing.B) {
	r := rand.New(rand.NewPCG(1, 2))
	rows, cols := 2048, 1024
	nb := cols / 32
	w := randBlocks(r, rows*nb, q80Bytes, []int{0})
	x := make([]float32, cols)
	for i := range x {
		x[i] = float32(r.NormFloat64())
	}
	y := make([]byte, nb*q80aBytes)
	quantizeQ8_0(x, y)
	rb := nb * q80Bytes
	b.SetBytes(int64(len(w)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < rows; j++ {
			dotQ80(w[j*rb:], y, nb)
		}
	}
}
