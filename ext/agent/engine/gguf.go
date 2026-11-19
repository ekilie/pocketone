package engine

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
)

// GGML tensor types used by GGUF.
const (
	typeF32  = 0
	typeF16  = 1
	typeQ4_0 = 2
	typeQ8_0 = 8
	typeQ4_K = 12
	typeQ5_K = 13
	typeQ6_K = 14
	typeQ8_K = 15
	typeBF16 = 30
)

var typeNames = map[uint32]string{
	typeF32: "F32", typeF16: "F16", typeQ4_0: "Q4_0", 3: "Q4_1", 6: "Q5_0", 7: "Q5_1",
	typeQ8_0: "Q8_0", 10: "Q2_K", 11: "Q3_K", typeQ4_K: "Q4_K", typeQ5_K: "Q5_K",
	typeQ6_K: "Q6_K", typeQ8_K: "Q8_K", typeBF16: "BF16",
}

func typeName(t uint32) string {
	if n, ok := typeNames[t]; ok {
		return n
	}
	return fmt.Sprintf("type%d", t)
}

// blockInfo returns elements per block and bytes per block.
func blockInfo(t uint32) (int, int, bool) {
	switch t {
	case typeF32:
		return 1, 4, true
	case typeF16, typeBF16:
		return 1, 2, true
	case typeQ8_0:
		return 32, 34, true
	case typeQ4_0:
		return 32, 18, true
	case typeQ4_K:
		return 256, 144, true
	case typeQ5_K:
		return 256, 176, true
	case typeQ6_K:
		return 256, 210, true
	case typeQ8_K:
		return 256, 292, true
	}
	return 0, 0, false
}

type tensorInfo struct {
	Name   string
	Dims   []uint64 // ggml order: Dims[0] is the row length (contiguous)
	Type   uint32
	Offset uint64
	Data   []byte
}

func (t *tensorInfo) rows() int {
	n := 1
	for _, d := range t.Dims[1:] {
		n *= int(d)
	}
	return n
}

type ggufFile struct {
	Version uint32
	KV      map[string]any
	Tensors map[string]*tensorInfo
	Order   []string
	data    []byte // whole mmapped file
	size    int64
}

type ggufReader struct {
	b   []byte
	pos int
	err error
}

func (r *ggufReader) need(n int) bool {
	if r.err != nil {
		return false
	}
	if r.pos+n > len(r.b) {
		r.err = errors.New("gguf: unexpected end of file")
		return false
	}
	return true
}

func (r *ggufReader) u8() uint8 {
	if !r.need(1) {
		return 0
	}
	v := r.b[r.pos]
	r.pos++
	return v
}
func (r *ggufReader) u16() uint16 {
	if !r.need(2) {
		return 0
	}
	v := binary.LittleEndian.Uint16(r.b[r.pos:])
	r.pos += 2
	return v
}
func (r *ggufReader) u32() uint32 {
	if !r.need(4) {
		return 0
	}
	v := binary.LittleEndian.Uint32(r.b[r.pos:])
	r.pos += 4
	return v
}
func (r *ggufReader) u64() uint64 {
	if !r.need(8) {
		return 0
	}
	v := binary.LittleEndian.Uint64(r.b[r.pos:])
	r.pos += 8
	return v
}
func (r *ggufReader) str() string {
	n := r.u64()
	if !r.need(int(n)) {
		return ""
	}
	s := string(r.b[r.pos : r.pos+int(n)])
	r.pos += int(n)
	return s
}

func (r *ggufReader) value(t uint32) any {
	switch t {
	case 0:
		return r.u8()
	case 1:
		return int8(r.u8())
	case 2:
		return r.u16()
	case 3:
		return int16(r.u16())
	case 4:
		return r.u32()
	case 5:
		return int32(r.u32())
	case 6:
		return math.Float32frombits(r.u32())
	case 7:
		return r.u8() != 0
	case 8:
		return r.str()
	case 9:
		et := r.u32()
		n := r.u64()
		if r.err != nil {
			return nil
		}
		if n > uint64(len(r.b)) {
			r.err = errors.New("gguf: bad array length")
			return nil
		}
		switch et {
		case 8:
			out := make([]string, n)
			for i := range out {
				out[i] = r.str()
			}
			return out
		case 5:
			out := make([]int32, n)
			for i := range out {
				out[i] = int32(r.u32())
			}
			return out
		case 6:
			out := make([]float32, n)
			for i := range out {
				out[i] = math.Float32frombits(r.u32())
			}
			return out
		default:
			out := make([]any, n)
			for i := range out {
				out[i] = r.value(et)
			}
			return out
		}
	case 10:
		return r.u64()
	case 11:
		return int64(r.u64())
	case 12:
		return math.Float64frombits(r.u64())
	}
	r.err = fmt.Errorf("gguf: unknown value type %d", t)
	return nil
}

func openGGUF(path string) (*ggufFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	data, err := mmapFile(f, st.Size())
	if err != nil {
		return nil, err
	}
	g, err := parseGGUF(data)
	if err != nil {
		munmap(data)
		return nil, err
	}
	g.size = st.Size()
	return g, nil
}

func parseGGUF(data []byte) (*ggufFile, error) {
	r := &ggufReader{b: data}
	if len(data) < 4 || string(data[:4]) != "GGUF" {
		return nil, errors.New("gguf: bad magic")
	}
	r.pos = 4
	g := &ggufFile{KV: map[string]any{}, Tensors: map[string]*tensorInfo{}, data: data}
	g.Version = r.u32()
	if g.Version < 2 {
		return nil, fmt.Errorf("gguf: unsupported version %d", g.Version)
	}
	nt := r.u64()
	nkv := r.u64()
	for i := uint64(0); i < nkv && r.err == nil; i++ {
		k := r.str()
		t := r.u32()
		g.KV[k] = r.value(t)
	}
	for i := uint64(0); i < nt && r.err == nil; i++ {
		ti := &tensorInfo{Name: r.str()}
		nd := r.u32()
		for j := uint32(0); j < nd; j++ {
			ti.Dims = append(ti.Dims, r.u64())
		}
		ti.Type = r.u32()
		ti.Offset = r.u64()
		g.Tensors[ti.Name] = ti
		g.Order = append(g.Order, ti.Name)
	}
	if r.err != nil {
		return nil, r.err
	}
	align := 32
	if a, ok := g.KV["general.alignment"].(uint32); ok && a > 0 {
		align = int(a)
	}
	base := (r.pos + align - 1) / align * align
	for _, ti := range g.Tensors {
		var n uint64 = 1
		for _, d := range ti.Dims {
			n *= d
		}
		be, bb, ok := blockInfo(ti.Type)
		if !ok {
			// size unknown; leave Data nil, error later if used
			continue
		}
		size := n / uint64(be) * uint64(bb)
		start := uint64(base) + ti.Offset
		if start+size > uint64(len(data)) {
			return nil, fmt.Errorf("gguf: tensor %s out of range", ti.Name)
		}
		ti.Data = data[start : start+size : start+size]
	}
	return g, nil
}

func (g *ggufFile) close() error {
	if g.data != nil {
		err := munmap(g.data)
		g.data = nil
		return err
	}
	return nil
}

func (g *ggufFile) getInt(key string) (int, bool) {
	switch v := g.KV[key].(type) {
	case uint32:
		return int(v), true
	case int32:
		return int(v), true
	case uint64:
		return int(v), true
	case int64:
		return int(v), true
	case uint16:
		return int(v), true
	case uint8:
		return int(v), true
	}
	return 0, false
}

func (g *ggufFile) getFloat(key string) (float64, bool) {
	switch v := g.KV[key].(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

func (g *ggufFile) getString(key string) string {
	s, _ := g.KV[key].(string)
	return s
}
