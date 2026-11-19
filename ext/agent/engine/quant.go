package engine

import (
	"encoding/binary"
	"math"
	"unsafe"
)

// Block sizes (bytes) of the formats we compute with.
const (
	qk         = 256 // elements per K-quant super-block
	q4kBytes   = 144 // d f16, dmin f16, scales[12], qs[128]
	q6kBytes   = 210 // ql[128], qh[64], scales[16] int8, d f16
	q8kBytes   = 292 // d f32, qs[256] int8, bsums[16] int16 (ggml block_q8_K)
	q8kQsOff   = 4
	q8kBsumOff = 260
)

var f16Table [1 << 16]float32

func init() {
	for i := range f16Table {
		f16Table[i] = f16ToF32Slow(uint16(i))
	}
}

func f16ToF32Slow(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := uint32(h>>10) & 0x1f
	mant := uint32(h) & 0x3ff
	switch {
	case exp == 0 && mant == 0:
		return math.Float32frombits(sign)
	case exp == 0: // subnormal
		e := uint32(127 - 15 + 1)
		for mant&0x400 == 0 {
			mant <<= 1
			e--
		}
		mant &= 0x3ff
		return math.Float32frombits(sign | e<<23 | mant<<13)
	case exp == 0x1f:
		return math.Float32frombits(sign | 0xff<<23 | mant<<13)
	}
	return math.Float32frombits(sign | (exp+127-15)<<23 | mant<<13)
}

func f16(b []byte) float32 { return f16Table[binary.LittleEndian.Uint16(b)] }

// scaleMinK4 decodes the 6-bit scale and min of sub-block j from a Q4_K/Q5_K scales array.
func scaleMinK4(j int, q []byte) (uint8, uint8) {
	if j < 4 {
		return q[j] & 63, q[j+4] & 63
	}
	return (q[j+4] & 0xF) | ((q[j-4] >> 6) << 4), (q[j+4] >> 4) | ((q[j] >> 6) << 4)
}

// dequantRow writes the n values of a row of type t into out.
func dequantRow(t uint32, row []byte, out []float32) {
	n := len(out)
	switch t {
	case typeF32:
		for i := range out {
			out[i] = math.Float32frombits(binary.LittleEndian.Uint32(row[4*i:]))
		}
	case typeF16:
		for i := range out {
			out[i] = f16(row[2*i:])
		}
	case typeQ8_0:
		for b := 0; b < n/32; b++ {
			blk := row[b*34:]
			d := f16(blk)
			for j := 0; j < 32; j++ {
				out[b*32+j] = d * float32(int8(blk[2+j]))
			}
		}
	case typeQ4_K:
		for b := 0; b < n/qk; b++ {
			blk := row[b*q4kBytes:]
			d, dmin := f16(blk), f16(blk[2:])
			sc := blk[4:16]
			qs := blk[16:144]
			y := out[b*qk:]
			for j := 0; j < 4; j++ {
				s1, m1 := scaleMinK4(2*j, sc)
				s2, m2 := scaleMinK4(2*j+1, sc)
				d1, mm1 := d*float32(s1), dmin*float32(m1)
				d2, mm2 := d*float32(s2), dmin*float32(m2)
				for l := 0; l < 32; l++ {
					q := qs[32*j+l]
					y[64*j+l] = d1*float32(q&0xF) - mm1
					y[64*j+32+l] = d2*float32(q>>4) - mm2
				}
			}
		}
	case typeQ6_K:
		for b := 0; b < n/qk; b++ {
			blk := row[b*q6kBytes:]
			d := f16(blk[208:])
			y := out[b*qk:]
			for h := 0; h < 2; h++ {
				ql := blk[64*h:]
				qh := blk[128+32*h:]
				sc := blk[192+8*h:]
				yy := y[128*h:]
				for l := 0; l < 32; l++ {
					is := l / 16
					q1 := int32(ql[l]&0xF|((qh[l]>>0)&3)<<4) - 32
					q2 := int32(ql[l+32]&0xF|((qh[l]>>2)&3)<<4) - 32
					q3 := int32(ql[l]>>4|((qh[l]>>4)&3)<<4) - 32
					q4 := int32(ql[l+32]>>4|((qh[l]>>6)&3)<<4) - 32
					yy[l] = d * float32(int8(sc[is])) * float32(q1)
					yy[l+32] = d * float32(int8(sc[is+2])) * float32(q2)
					yy[l+64] = d * float32(int8(sc[is+4])) * float32(q3)
					yy[l+96] = d * float32(int8(sc[is+6])) * float32(q4)
				}
			}
		}
	default:
		panic("dequantRow: unsupported type " + typeName(t))
	}
}

// quantizeQ8K quantizes x (len multiple of 256) into ggml block_q8_K layout.
func quantizeQ8K(x []float32, out []byte) {
	for b := 0; b < len(x)/qk; b++ {
		xb := x[b*qk : b*qk+qk]
		ob := out[b*q8kBytes : b*q8kBytes+q8kBytes]
		var amax, max float32
		for _, v := range xb {
			if a := float32(math.Abs(float64(v))); a > amax {
				amax, max = a, v
			}
		}
		if amax == 0 {
			for i := range ob {
				ob[i] = 0
			}
			continue
		}
		iscale := -128 / max
		for j, v := range xb {
			q := int32(math.RoundToEven(float64(iscale * v)))
			if q > 127 {
				q = 127
			}
			if q < -128 {
				q = -128
			}
			ob[q8kQsOff+j] = byte(int8(q))
		}
		for j := 0; j < 16; j++ {
			var s int32
			for l := 0; l < 16; l++ {
				s += int32(int8(ob[q8kQsOff+16*j+l]))
			}
			binary.LittleEndian.PutUint16(ob[q8kBsumOff+2*j:], uint16(int16(s)))
		}
		binary.LittleEndian.PutUint32(ob, math.Float32bits(1/iscale))
	}
}

func dotQ4KQ8KGo(x, y []byte, nb int) float32 {
	var sum float32
	for i := 0; i < nb; i++ {
		xb := x[i*q4kBytes:]
		yb := y[i*q8kBytes:]
		yd := math.Float32frombits(binary.LittleEndian.Uint32(yb))
		d := f16(xb) * yd
		dmin := f16(xb[2:]) * yd
		sc := xb[4:16]
		qs := xb[16:144]
		q8 := yb[q8kQsOff:]
		var sumi, sumMins int32
		for j := 0; j < 4; j++ {
			s1, m1 := scaleMinK4(2*j, sc)
			s2, m2 := scaleMinK4(2*j+1, sc)
			var a, b int32
			for l := 0; l < 32; l++ {
				q := qs[32*j+l]
				a += int32(q&0xF) * int32(int8(q8[64*j+l]))
				b += int32(q>>4) * int32(int8(q8[64*j+32+l]))
			}
			sumi += a*int32(s1) + b*int32(s2)
			bs := func(k int) int32 { return int32(int16(binary.LittleEndian.Uint16(yb[q8kBsumOff+2*k:]))) }
			sumMins += int32(m1)*(bs(4*j)+bs(4*j+1)) + int32(m2)*(bs(4*j+2)+bs(4*j+3))
		}
		sum += d*float32(sumi) - dmin*float32(sumMins)
	}
	return sum
}

func dotQ6KQ8KGo(x, y []byte, nb int) float32 {
	var sum float32
	for i := 0; i < nb; i++ {
		xb := x[i*q6kBytes:]
		yb := y[i*q8kBytes:]
		d := f16(xb[208:]) * math.Float32frombits(binary.LittleEndian.Uint32(yb))
		q8 := yb[q8kQsOff:]
		var sumi int32
		for h := 0; h < 2; h++ {
			ql := xb[64*h:]
			qh := xb[128+32*h:]
			sc := xb[192+8*h:]
			q := q8[128*h:]
			var s [8]int32
			for l := 0; l < 32; l++ {
				is := l / 16
				q1 := int32(ql[l]&0xF|((qh[l]>>0)&3)<<4) - 32
				q2 := int32(ql[l+32]&0xF|((qh[l]>>2)&3)<<4) - 32
				q3 := int32(ql[l]>>4|((qh[l]>>4)&3)<<4) - 32
				q4 := int32(ql[l+32]>>4|((qh[l]>>6)&3)<<4) - 32
				s[is+0] += q1 * int32(int8(q[l]))
				s[is+2] += q2 * int32(int8(q[l+32]))
				s[is+4] += q3 * int32(int8(q[l+64]))
				s[is+6] += q4 * int32(int8(q[l+96]))
			}
			for k := 0; k < 8; k++ {
				sumi += s[k] * int32(int8(sc[k]))
			}
		}
		sum += d * float32(sumi)
	}
	return sum
}

func dotF16F32Go(x []byte, y []float32) float32 {
	var s0, s1, s2, s3 float32
	n := len(y)
	i := 0
	for ; i+4 <= n; i += 4 {
		s0 += f16(x[2*i:]) * y[i]
		s1 += f16(x[2*i+2:]) * y[i+1]
		s2 += f16(x[2*i+4:]) * y[i+2]
		s3 += f16(x[2*i+6:]) * y[i+3]
	}
	for ; i < n; i++ {
		s0 += f16(x[2*i:]) * y[i]
	}
	return s0 + s1 + s2 + s3
}

func dotF32Go(x, y []float32) float32 {
	var s0, s1, s2, s3 float32
	n := len(x)
	y = y[:n]
	i := 0
	for ; i+4 <= n; i += 4 {
		s0 += x[i] * y[i]
		s1 += x[i+1] * y[i+1]
		s2 += x[i+2] * y[i+2]
		s3 += x[i+3] * y[i+3]
	}
	for ; i < n; i++ {
		s0 += x[i] * y[i]
	}
	return s0 + s1 + s2 + s3
}

func axpyGo(a float32, x, y []float32) {
	y = y[:len(x)]
	for i, v := range x {
		y[i] += a * v
	}
}

func bptr(b []byte) unsafe.Pointer    { return unsafe.Pointer(unsafe.SliceData(b)) }
func fptr(f []float32) unsafe.Pointer { return unsafe.Pointer(unsafe.SliceData(f)) }
