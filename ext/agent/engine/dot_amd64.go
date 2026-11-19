//go:build amd64

package engine

// cpuid and xgetbv are in dot_amd64.s.
func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)
func xgetbv() (eax, edx uint32)

// asmFeatures: every amd64 kernel needs AVX2 and FMA (and F16C for the F16
// weights), with the OS saving YMM state. Without them the Go kernels run.
func asmFeatures() (float, ints bool, name string) {
	if hasAVX2FMA() {
		return true, true, "avx2+fma"
	}
	return false, false, ""
}

func hasAVX2FMA() bool {
	maxID, _, _, _ := cpuid(0, 0)
	if maxID < 7 {
		return false
	}
	_, _, ecx1, _ := cpuid(1, 0)
	const (
		fma     = 1 << 12
		osxsave = 1 << 27
		avx     = 1 << 28
		f16c    = 1 << 29
	)
	if ecx1&(fma|osxsave|avx|f16c) != fma|osxsave|avx|f16c {
		return false
	}
	if xcr0, _ := xgetbv(); xcr0&6 != 6 { // XMM and YMM state enabled by the OS
		return false
	}
	_, ebx7, _, _ := cpuid(7, 0)
	return ebx7&(1<<5) != 0 // AVX2
}
