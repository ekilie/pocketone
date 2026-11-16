//go:build 386 || arm

package jsvm

import (
	"math"

	lib "modernc.org/libquickjs"
)

// On 32-bit platforms (libquickjs translates linux/386 and linux/arm this way) QuickJS NaN-boxes a JSValue into a
// uint64: the tag in the high 32 bits and the payload (an int, a bool, a pointer) in the low 32. A float is stored as
// its bits minus floatAddend<<32, so its high word falls outside the tag range. These mirror quickjs.h's JS_MKVAL,
// __JS_NewFloat64, JS_VALUE_GET_FLOAT64 and JS_VALUE_GET_NORM_TAG for JS_NAN_BOXING.

// floatAddend is quickjs.h's JS_FLOAT64_TAG_ADDEND.
const floatAddend = uint64(uint32(int32(0x7ff80000) - int32(lib.EJS_TAG_FIRST) + 1))

// mkval is a value with tag and its payload's low 32 bits (an int, a bool, or 0); floats use mkfloat.
func mkval(tag int32, bits uint64) JSValue {
	return JSValue(uint64(uint32(tag))<<32 | uint64(uint32(bits)))
}

// mkfloat is a float64 value; every NaN becomes QuickJS's one canonical NaN.
func mkfloat(d float64) JSValue {
	u := math.Float64bits(d)
	if u&0x7fffffffffffffff > 0x7ff0000000000000 {
		u = 0x7ff8000000000000
	}
	return JSValue(u - floatAddend<<32)
}

// valTag is v's tag, JS_TAG_FLOAT64 for any float (JS_VALUE_GET_NORM_TAG).
func valTag(v JSValue) int32 {
	tag := int32(uint64(v) >> 32)
	if uint32(tag-lib.EJS_TAG_FIRST) >= uint32(lib.EJS_TAG_FLOAT64-lib.EJS_TAG_FIRST) {
		return lib.EJS_TAG_FLOAT64
	}
	return tag
}

// valPtr is the pointer a pointer-tagged value (object, module, ...) carries.
func valPtr(v JSValue) uintptr { return uintptr(uint32(uint64(v))) }
