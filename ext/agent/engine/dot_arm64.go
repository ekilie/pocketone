//go:build arm64

package engine

import (
	"encoding/binary"
	"os"
	"runtime"
)

// asmFeatures: base NEON (float kernels) is always present on arm64; the
// 8-bit kernels need the ARMv8.2 SDOT instruction.
func asmFeatures() (float, ints bool, name string) {
	if hasDotProd() {
		return true, true, "neon+dotprod"
	}
	return true, false, "neon"
}

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
