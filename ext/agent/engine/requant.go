package engine

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sort"
)

// requantizeQ8_0 writes a copy of the GGUF file at in to out with every 2-D
// weight converted to Q8_0 (norms stay F32). It exists for testing and
// benchmarking the Q8_0 path from a model already on disk; converting from a
// 4-bit file does not recover the original precision.
func requantizeQ8_0(in, out string) error {
	g, err := openGGUF(in)
	if err != nil {
		return err
	}
	defer g.close()
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 1<<20)
	le := binary.LittleEndian
	wu32 := func(v uint32) { binary.Write(w, le, v) }
	wu64 := func(v uint64) { binary.Write(w, le, v) }
	wstr := func(s string) { wu64(uint64(len(s))); w.WriteString(s) }

	type outT struct {
		ti   *tensorInfo
		typ  uint32
		size uint64
		off  uint64
	}
	var ts []outT
	var off uint64
	align := uint64(32)
	for _, name := range g.Order {
		ti := g.Tensors[name]
		typ := ti.Type
		n := uint64(1)
		for _, d := range ti.Dims {
			n *= d
		}
		if len(ti.Dims) == 2 && ti.Dims[0]%32 == 0 && typ != typeF32 {
			typ = typeQ8_0
		}
		be, bb, ok := blockInfo(typ)
		if !ok {
			return fmt.Errorf("requant: unsupported tensor type %s", typeName(typ))
		}
		size := n / uint64(be) * uint64(bb)
		ts = append(ts, outT{ti, typ, size, off})
		off = (off + size + align - 1) / align * align
	}

	w.WriteString("GGUF")
	wu32(3)
	wu64(uint64(len(ts)))
	keys := make([]string, 0, len(g.KV))
	for k := range g.KV {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	wu64(uint64(len(keys)))
	for _, k := range keys {
		v := g.KV[k]
		if k == "general.file_type" {
			v = uint32(7) // MOSTLY_Q8_0
		}
		if k == "general.alignment" {
			v = uint32(align)
		}
		wstr(k)
		switch x := v.(type) {
		case uint8:
			wu32(0)
			w.WriteByte(x)
		case int8:
			wu32(1)
			w.WriteByte(byte(x))
		case uint16:
			wu32(2)
			binary.Write(w, le, x)
		case int16:
			wu32(3)
			binary.Write(w, le, x)
		case uint32:
			wu32(4)
			wu32(x)
		case int32:
			wu32(5)
			binary.Write(w, le, x)
		case float32:
			wu32(6)
			binary.Write(w, le, x)
		case bool:
			wu32(7)
			if x {
				w.WriteByte(1)
			} else {
				w.WriteByte(0)
			}
		case string:
			wu32(8)
			wstr(x)
		case uint64:
			wu32(10)
			wu64(x)
		case int64:
			wu32(11)
			binary.Write(w, le, x)
		case float64:
			wu32(12)
			binary.Write(w, le, x)
		case []string:
			wu32(9)
			wu32(8)
			wu64(uint64(len(x)))
			for _, s := range x {
				wstr(s)
			}
		case []int32:
			wu32(9)
			wu32(5)
			wu64(uint64(len(x)))
			binary.Write(w, le, x)
		case []float32:
			wu32(9)
			wu32(6)
			wu64(uint64(len(x)))
			binary.Write(w, le, x)
		default:
			return fmt.Errorf("requant: cannot write metadata %s (%T)", k, v)
		}
	}
	for _, t := range ts {
		wstr(t.ti.Name)
		wu32(uint32(len(t.ti.Dims)))
		for _, d := range t.ti.Dims {
			wu64(d)
		}
		wu32(t.typ)
		wu64(t.off)
	}
	w.Flush()
	pos, _ := f.Seek(0, 1)
	pad := func(to uint64) {
		for uint64(pos) < to {
			w.WriteByte(0)
			pos++
		}
	}
	base := (uint64(pos) + align - 1) / align * align
	pad(base)
	for _, t := range ts {
		pad(base + t.off)
		if t.typ == t.ti.Type {
			w.Write(t.ti.Data)
			pos += int64(len(t.ti.Data))
			continue
		}
		cols := int(t.ti.Dims[0])
		rows := t.ti.rows()
		rb := len(t.ti.Data) / rows
		row := make([]float32, cols)
		blk := make([]byte, cols/32*q80Bytes)
		for r := 0; r < rows; r++ {
			dequantRow(t.ti.Type, t.ti.Data[r*rb:(r+1)*rb], row)
			quantizeWeightQ8_0(row, blk)
			w.Write(blk)
			pos += int64(len(blk))
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Close()
}

// quantizeWeightQ8_0 quantizes x into ggml block_q8_0 (d as f16).
func quantizeWeightQ8_0(x []float32, out []byte) {
	for b := 0; b < len(x)/32; b++ {
		xb := x[b*32 : b*32+32]
		ob := out[b*q80Bytes:]
		var amax float32
		for _, v := range xb {
			amax = max(amax, float32(math.Abs(float64(v))))
		}
		d := amax / 127
		id := float32(0)
		if d != 0 {
			id = 1 / d
		}
		binary.LittleEndian.PutUint16(ob, f32ToF16(d))
		for j, v := range xb {
			ob[2+j] = byte(int8(math.RoundToEven(float64(v * id))))
		}
	}
}

// f32ToF16 converts with round-to-nearest-even (normal range and subnormals).
func f32ToF16(f float32) uint16 {
	b := math.Float32bits(f)
	sign := uint16(b>>16) & 0x8000
	exp := int32(b>>23&0xff) - 127 + 15
	mant := b & 0x7fffff
	switch {
	case exp >= 31:
		return sign | 0x7c00
	case exp <= 0:
		if exp < -10 {
			return sign
		}
		mant |= 0x800000
		shift := uint32(14 - exp)
		h := mant >> shift
		rem := mant & (1<<shift - 1)
		halfway := uint32(1) << (shift - 1)
		if rem > halfway || (rem == halfway && h&1 == 1) {
			h++
		}
		return sign | uint16(h)
	}
	h := uint32(exp)<<10 | mant>>13
	rem := mant & 0x1fff
	if rem > 0x1000 || (rem == 0x1000 && h&1 == 1) {
		h++
	}
	return sign | uint16(h)
}
