//go:build !386 && !arm

package jsvm

import (
	"math"

	lib "modernc.org/libquickjs"
)

// On 64-bit platforms a JSValue is QuickJS's 16-byte struct: a union payload and a tag.

// mkval is a value with tag and its payload's raw bits (an int, a bool, or 0); floats use mkfloat.
func mkval(tag int32, bits uint64) JSValue {
	var v JSValue
	v.Ftag = int64(tag)
	v.Fu.Fuint641 = bits
	return v
}

// mkfloat is a float64 value.
func mkfloat(d float64) JSValue { return mkval(lib.EJS_TAG_FLOAT64, math.Float64bits(d)) }

// valTag is v's tag (JS_TAG_FLOAT64 for any float).
func valTag(v JSValue) int32 { return int32(v.Ftag) }

// valPtr is the pointer a pointer-tagged value (object, module, ...) carries.
func valPtr(v JSValue) uintptr { return uintptr(v.Fu.Fuint641) }
