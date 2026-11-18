//go:build arm64

#include "textflag.h"

// Instructions the Go assembler doesn't know, hand-encoded. Arguments are register numbers.
#define ZERO(d) WORD $(0x6F00E400 | (d))                                        // MOVI Vd.2D, #0
#define SDOT(m, n, d) WORD $(0x4E809400 | ((m)<<16) | ((n)<<5) | (d))          // Vd.4S += Vn.16B . Vm.16B
#define SCVTF4S(n, d) WORD $(0x4E21D800 | ((n)<<5) | (d))                       // Vd.4S = float(Vn.4S)
#define FADDP4S(m, n, d) WORD $(0x6E20D400 | ((m)<<16) | ((n)<<5) | (d))
#define FADDP2S(n, d) WORD $(0x7E30D800 | ((n)<<5) | (d))                       // Sd = Vn.S[0]+Vn.S[1]
#define MLAE(m, i, n, d) WORD $(0x6F800000 | (((i)&1)<<21) | (((m)&15)<<16) | (((i)>>1)<<11) | ((n)<<5) | (d)) // Vd.4S += Vn.4S * Vm.S[i], m<16
#define SMULL4H(m, n, d) WORD $(0x0E60C000 | ((m)<<16) | ((n)<<5) | (d))        // Vd.4S = Vn.4H * Vm.4H
#define SMLAL8H(m, n, d) WORD $(0x4E608000 | ((m)<<16) | ((n)<<5) | (d))        // Vd.4S += Vn.8H[4:] * Vm.8H[4:]
#define ADDP8H(m, n, d) WORD $(0x4E60BC00 | ((m)<<16) | ((n)<<5) | (d))
#define UXTL8B(n, d) WORD $(0x2F08A400 | ((n)<<5) | (d))
#define UXTL4H(n, d) WORD $(0x2F10A400 | ((n)<<5) | (d))
#define UXTL2_8H(n, d) WORD $(0x6F10A400 | ((n)<<5) | (d))
#define SXTL8B(n, d) WORD $(0x0F08A400 | ((n)<<5) | (d))
#define SXTL2_16B(n, d) WORD $(0x4F08A400 | ((n)<<5) | (d))
#define SXTL4H(n, d) WORD $(0x0F10A400 | ((n)<<5) | (d))
#define SXTL2_8H(n, d) WORD $(0x4F10A400 | ((n)<<5) | (d))
#define FCVTL4H(n, d) WORD $(0x0E217800 | ((n)<<5) | (d))
#define FCVTL2_8H(n, d) WORD $(0x4E217800 | ((n)<<5) | (d))

// func dotQ4KQ8KAsm(x, y unsafe.Pointer, nb int) float32
TEXT ·dotQ4KQ8KAsm(SB), NOSPLIT, $0-28
	MOVD x+0(FP), R0
	MOVD y+8(FP), R1
	MOVD nb+16(FP), R2
	MOVW $0x0f0f0f0f, R3
	VDUP R3, V31.S4
	VMOVI $0, V30.B16 // float accumulator
	CBZ  R2, q4done

q4loop:
	// d, dmin (f16) times y.d
	MOVHU (R0), R3
	MOVHU 2(R0), R4
	FMOVS (R1), F2
	FMOVS R3, F3
	FCVTHS F3, F3
	FMULS F2, F3, F3
	FMOVS R4, F4
	FCVTHS F4, F4
	FMULS F2, F4, F4
	VDUP V3.S[0], V26.S4  // d
	VDUP V4.S[0], V27.S4  // dmin

	// decode 6-bit scales and mins (ggml utmp trick)
	MOVWU 4(R0), R5
	MOVWU 8(R0), R6
	MOVWU 12(R0), R7
	LSRW $4, R7, R8
	ANDW $0x0f0f0f0f, R8, R8
	LSRW $6, R6, R9
	ANDW $0x03030303, R9, R9
	LSLW $4, R9, R9
	ORRW R9, R8, R8               // utmp3
	ANDW $0x3f3f3f3f, R6, R10     // utmp2
	ANDW $0x0f0f0f0f, R7, R11
	LSRW $6, R5, R9
	ANDW $0x03030303, R9, R9
	LSLW $4, R9, R9
	ORRW R9, R11, R11             // utmp1
	ANDW $0x3f3f3f3f, R5, R12     // utmp0
	ORR  R11<<32, R12, R12        // scales[0..7]
	ORR  R8<<32, R10, R10         // mins[0..7]
	VMOV R12, V0.D[0]
	VMOV R10, V1.D[0]
	UXTL8B(0, 0)                  // V0.8H scales
	UXTL8B(1, 1)                  // V1.8H mins
	UXTL4H(0, 2)                  // V2.4S scales 0-3
	UXTL2_8H(0, 3)                // V3.4S scales 4-7

	// mins . bsums
	ADD  $260, R1, R13
	VLD1 (R13), [V4.H8, V5.H8]
	ADDP8H(5, 4, 6)               // V6.8H = pairwise sums of 16 bsums
	SMULL4H(1, 6, 7)
	SMLAL8H(1, 6, 7)              // V7.4S
	SCVTF4S(7, 7)
	VFMLS V27.S4, V7.S4, V30.S4   // acc -= dmin * mins

	ADD  $16, R0, R14
	ADD  $4, R1, R15
	VMOVI $0, V16.B16

	// 4 x 64 elements
	VLD1.P 32(R14), [V8.B16, V9.B16]
	VLD1.P 64(R15), [V10.B16, V11.B16, V12.B16, V13.B16]
	VAND V31.B16, V8.B16, V14.B16
	VAND V31.B16, V9.B16, V15.B16
	VUSHR $4, V8.B16, V8.B16
	VUSHR $4, V9.B16, V9.B16
	VMOVI $0, V17.B16
	VMOVI $0, V18.B16
	SDOT(10, 14, 17)
	SDOT(11, 15, 17)
	SDOT(12, 8, 18)
	SDOT(13, 9, 18)
	MLAE(2, 0, 17, 16)
	MLAE(2, 1, 18, 16)

	VLD1.P 32(R14), [V8.B16, V9.B16]
	VLD1.P 64(R15), [V10.B16, V11.B16, V12.B16, V13.B16]
	VAND V31.B16, V8.B16, V14.B16
	VAND V31.B16, V9.B16, V15.B16
	VUSHR $4, V8.B16, V8.B16
	VUSHR $4, V9.B16, V9.B16
	VMOVI $0, V19.B16
	VMOVI $0, V20.B16
	SDOT(10, 14, 19)
	SDOT(11, 15, 19)
	SDOT(12, 8, 20)
	SDOT(13, 9, 20)
	MLAE(2, 2, 19, 16)
	MLAE(2, 3, 20, 16)

	VLD1.P 32(R14), [V8.B16, V9.B16]
	VLD1.P 64(R15), [V10.B16, V11.B16, V12.B16, V13.B16]
	VAND V31.B16, V8.B16, V14.B16
	VAND V31.B16, V9.B16, V15.B16
	VUSHR $4, V8.B16, V8.B16
	VUSHR $4, V9.B16, V9.B16
	VMOVI $0, V17.B16
	VMOVI $0, V18.B16
	SDOT(10, 14, 17)
	SDOT(11, 15, 17)
	SDOT(12, 8, 18)
	SDOT(13, 9, 18)
	MLAE(3, 0, 17, 16)
	MLAE(3, 1, 18, 16)

	VLD1.P 32(R14), [V8.B16, V9.B16]
	VLD1.P 64(R15), [V10.B16, V11.B16, V12.B16, V13.B16]
	VAND V31.B16, V8.B16, V14.B16
	VAND V31.B16, V9.B16, V15.B16
	VUSHR $4, V8.B16, V8.B16
	VUSHR $4, V9.B16, V9.B16
	VMOVI $0, V19.B16
	VMOVI $0, V20.B16
	SDOT(10, 14, 19)
	SDOT(11, 15, 19)
	SDOT(12, 8, 20)
	SDOT(13, 9, 20)
	MLAE(3, 2, 19, 16)
	MLAE(3, 3, 20, 16)

	SCVTF4S(16, 16)
	VFMLA V26.S4, V16.S4, V30.S4  // acc += d * sumi

	ADD  $144, R0
	ADD  $292, R1
	SUBS $1, R2
	BNE  q4loop

q4done:
	FADDP4S(30, 30, 30)
	FADDP2S(30, 0)
	FMOVS F0, ret+24(FP)
	RET

// One 64-element half-chunk of a Q6_K block.
// ql in V8..V11 (or shifted), hi bits prepared in V14,V15,V17,V18; q8 loaded into V20..V23.
#define Q6PART(sc) \
	VORR V14.B16, V24.B16, V24.B16 \
	VORR V15.B16, V25.B16, V25.B16 \
	VORR V17.B16, V26.B16, V26.B16 \
	VORR V18.B16, V27.B16, V27.B16 \
	VSUB V30.B16, V24.B16, V24.B16 \
	VSUB V30.B16, V25.B16, V25.B16 \
	VSUB V30.B16, V26.B16, V26.B16 \
	VSUB V30.B16, V27.B16, V27.B16 \
	VMOVI $0, V14.B16 \
	VMOVI $0, V15.B16 \
	VMOVI $0, V17.B16 \
	VMOVI $0, V18.B16 \
	SDOT(20, 24, 14) \
	SDOT(21, 25, 15) \
	SDOT(22, 26, 17) \
	SDOT(23, 27, 18) \
	MLAE(sc, 0, 14, 16) \
	MLAE(sc, 1, 15, 16) \
	MLAE(sc, 2, 17, 16) \
	MLAE(sc, 3, 18, 16)

#define Q6HALF(scA, scB) \
	VLD1.P 64(R4), [V8.B16, V9.B16, V10.B16, V11.B16] \
	VLD1.P 32(R5), [V12.B16, V13.B16] \
	VLD1.P 64(R6), [V20.B16, V21.B16, V22.B16, V23.B16] \
	VSHL $4, V12.B16, V14.B16 \
	VAND V29.B16, V14.B16, V14.B16 \
	VSHL $4, V13.B16, V15.B16 \
	VAND V29.B16, V15.B16, V15.B16 \
	VSHL $2, V12.B16, V17.B16 \
	VAND V29.B16, V17.B16, V17.B16 \
	VSHL $2, V13.B16, V18.B16 \
	VAND V29.B16, V18.B16, V18.B16 \
	VAND V28.B16, V8.B16, V24.B16 \
	VAND V28.B16, V9.B16, V25.B16 \
	VAND V28.B16, V10.B16, V26.B16 \
	VAND V28.B16, V11.B16, V27.B16 \
	Q6PART(scA) \
	VLD1.P 64(R6), [V20.B16, V21.B16, V22.B16, V23.B16] \
	VAND V29.B16, V12.B16, V14.B16 \
	VAND V29.B16, V13.B16, V15.B16 \
	VUSHR $2, V12.B16, V17.B16 \
	VAND V29.B16, V17.B16, V17.B16 \
	VUSHR $2, V13.B16, V18.B16 \
	VAND V29.B16, V18.B16, V18.B16 \
	VUSHR $4, V8.B16, V24.B16 \
	VUSHR $4, V9.B16, V25.B16 \
	VUSHR $4, V10.B16, V26.B16 \
	VUSHR $4, V11.B16, V27.B16 \
	Q6PART(scB)

// func dotQ6KQ8KAsm(x, y unsafe.Pointer, nb int) float32
TEXT ·dotQ6KQ8KAsm(SB), NOSPLIT, $0-28
	MOVD x+0(FP), R0
	MOVD y+8(FP), R1
	MOVD nb+16(FP), R2
	MOVW $0x0f0f0f0f, R3
	VDUP R3, V28.S4
	MOVW $0x30303030, R3
	VDUP R3, V29.S4
	MOVW $0x20202020, R3
	VDUP R3, V30.S4
	VMOVI $0, V31.B16
	CBZ  R2, q6done

q6loop:
	MOVHU 208(R0), R3
	FMOVS (R1), F2
	FMOVS R3, F7
	FCVTHS F7, F7
	FMULS F2, F7, F7
	VDUP V7.S[0], V7.S4

	ADD  $192, R0, R3
	VLD1 (R3), [V0.B16]
	SXTL8B(0, 1)
	SXTL2_16B(0, 2)
	SXTL4H(1, 3)
	SXTL2_8H(1, 4)
	SXTL4H(2, 5)
	SXTL2_8H(2, 6)

	MOVD R0, R4
	ADD  $128, R0, R5
	ADD  $4, R1, R6
	VMOVI $0, V16.B16

	Q6HALF(3, 4)
	Q6HALF(5, 6)

	SCVTF4S(16, 16)
	VFMLA V7.S4, V16.S4, V31.S4

	ADD  $210, R0
	ADD  $292, R1
	SUBS $1, R2
	BNE  q6loop

q6done:
	FADDP4S(31, 31, 31)
	FADDP2S(31, 0)
	FMOVS F0, ret+24(FP)
	RET

// func dotF16F32Asm(x, y unsafe.Pointer, n int) float32   (n multiple of 16)
TEXT ·dotF16F32Asm(SB), NOSPLIT, $0-28
	MOVD x+0(FP), R0
	MOVD y+8(FP), R1
	MOVD n+16(FP), R2
	LSR  $4, R2
	VMOVI $0, V16.B16
	VMOVI $0, V17.B16
	VMOVI $0, V18.B16
	VMOVI $0, V19.B16
	CBZ  R2, f16done
f16loop:
	VLD1.P 32(R0), [V0.H8, V1.H8]
	VLD1.P 64(R1), [V4.S4, V5.S4, V6.S4, V7.S4]
	FCVTL4H(0, 2)
	FCVTL2_8H(0, 3)
	FCVTL4H(1, 8)
	FCVTL2_8H(1, 9)
	VFMLA V2.S4, V4.S4, V16.S4
	VFMLA V3.S4, V5.S4, V17.S4
	VFMLA V8.S4, V6.S4, V18.S4
	VFMLA V9.S4, V7.S4, V19.S4
	SUBS $1, R2
	BNE  f16loop
f16done:
	VFADD V17.S4, V16.S4, V16.S4
	VFADD V19.S4, V18.S4, V18.S4
	VFADD V18.S4, V16.S4, V16.S4
	FADDP4S(16, 16, 16)
	FADDP2S(16, 0)
	FMOVS F0, ret+24(FP)
	RET

// func dotF32Asm(x, y unsafe.Pointer, n int) float32   (n multiple of 16)
TEXT ·dotF32Asm(SB), NOSPLIT, $0-28
	MOVD x+0(FP), R0
	MOVD y+8(FP), R1
	MOVD n+16(FP), R2
	LSR  $4, R2
	VMOVI $0, V16.B16
	VMOVI $0, V17.B16
	VMOVI $0, V18.B16
	VMOVI $0, V19.B16
	CBZ  R2, f32done
f32loop:
	VLD1.P 64(R0), [V0.S4, V1.S4, V2.S4, V3.S4]
	VLD1.P 64(R1), [V4.S4, V5.S4, V6.S4, V7.S4]
	VFMLA V0.S4, V4.S4, V16.S4
	VFMLA V1.S4, V5.S4, V17.S4
	VFMLA V2.S4, V6.S4, V18.S4
	VFMLA V3.S4, V7.S4, V19.S4
	SUBS $1, R2
	BNE  f32loop
f32done:
	VFADD V17.S4, V16.S4, V16.S4
	VFADD V19.S4, V18.S4, V18.S4
	VFADD V18.S4, V16.S4, V16.S4
	FADDP4S(16, 16, 16)
	FADDP2S(16, 0)
	FMOVS F0, ret+24(FP)
	RET

// func axpyAsm(a float32, x, y unsafe.Pointer, n int)   y += a*x, n multiple of 16
TEXT ·axpyAsm(SB), NOSPLIT, $0-32
	FMOVS a+0(FP), F0
	MOVD x+8(FP), R0
	MOVD y+16(FP), R1
	MOVD n+24(FP), R2
	LSR  $4, R2
	VDUP V0.S[0], V31.S4
	CBZ  R2, axdone
	MOVD R1, R3
axloop:
	VLD1.P 64(R0), [V0.S4, V1.S4, V2.S4, V3.S4]
	VLD1.P 64(R1), [V4.S4, V5.S4, V6.S4, V7.S4]
	VFMLA V31.S4, V0.S4, V4.S4
	VFMLA V31.S4, V1.S4, V5.S4
	VFMLA V31.S4, V2.S4, V6.S4
	VFMLA V31.S4, V3.S4, V7.S4
	VST1.P [V4.S4, V5.S4, V6.S4, V7.S4], 64(R3)
	SUBS $1, R2
	BNE  axloop
axdone:
	RET

// func dotQ8_0Asm(x, y unsafe.Pointer, nb int) float32
// x: ggml block_q8_0 (d f16, qs[32]); y: activation blocks (d f32, qs[32]), 36 bytes.
// Two blocks per iteration into two accumulators (the FMLA chain is the bottleneck otherwise).
TEXT ·dotQ8_0Asm(SB), NOSPLIT, $0-28
	MOVD x+0(FP), R0
	MOVD y+8(FP), R1
	MOVD nb+16(FP), R2
	VMOVI $0, V30.B16
	VMOVI $0, V29.B16
	LSR  $1, R2, R6
	AND  $1, R2, R7
	CBZ  R6, q80tail
q80loop:
	MOVHU (R0), R3
	MOVHU 34(R0), R4
	FMOVS (R1), F2
	FMOVS 36(R1), F12
	FMOVS R3, F3
	FCVTHS F3, F3
	FMOVS R4, F13
	FCVTHS F13, F13
	FMULS F2, F3, F3
	FMULS F12, F13, F13
	VDUP V3.S[0], V3.S4
	VDUP V13.S[0], V13.S4
	ADD  $2, R0, R4
	ADD  $4, R1, R5
	VLD1 (R4), [V0.B16, V1.B16]
	VLD1 (R5), [V4.B16, V5.B16]
	ADD  $36, R0, R4
	ADD  $40, R1, R5
	VLD1 (R4), [V10.B16, V11.B16]
	VLD1 (R5), [V14.B16, V15.B16]
	VMOVI $0, V16.B16
	VMOVI $0, V17.B16
	SDOT(4, 0, 16)
	SDOT(14, 10, 17)
	SDOT(5, 1, 16)
	SDOT(15, 11, 17)
	SCVTF4S(16, 16)
	SCVTF4S(17, 17)
	VFMLA V3.S4, V16.S4, V30.S4
	VFMLA V13.S4, V17.S4, V29.S4
	ADD  $68, R0
	ADD  $72, R1
	SUBS $1, R6
	BNE  q80loop
q80tail:
	CBZ  R7, q80done
	MOVHU (R0), R3
	FMOVS (R1), F2
	FMOVS R3, F3
	FCVTHS F3, F3
	FMULS F2, F3, F3
	VDUP V3.S[0], V3.S4
	ADD  $2, R0, R4
	ADD  $4, R1, R5
	VLD1 (R4), [V0.B16, V1.B16]
	VLD1 (R5), [V4.B16, V5.B16]
	VMOVI $0, V16.B16
	SDOT(4, 0, 16)
	SDOT(5, 1, 16)
	SCVTF4S(16, 16)
	VFMLA V3.S4, V16.S4, V30.S4
q80done:
	VFADD V29.S4, V30.S4, V30.S4
	FADDP4S(30, 30, 30)
	FADDP2S(30, 0)
	FMOVS F0, ret+24(FP)
	RET

// ---- 4-column kernels: one weight row against 4 activation rows (prefill) ----

#define ADDP4S(m, n, d) WORD $(0x4EA0BC00 | ((m)<<16) | ((n)<<5) | (d))
#define FMULE0(m, n, d) WORD $(0x4F809000 | (((m)>>4)<<20) | (((m)&15)<<16) | ((n)<<5) | (d)) // Vd.4S = Vn.4S * Vm.S[0]
#define VFMUL4S(m, n, d) WORD $(0x6E20DC00 | ((m)<<16) | ((n)<<5) | (d))

// Q4x4COL(P, A): column with q8 pointer P, int accumulator A (register number), scale reg S, indices i0,i1.
#define Q4x4COL(PTR, A, S, i0, i1, T0, T1) \
	VLD1.P 64(PTR), [V4.B16, V5.B16, V6.B16, V7.B16] \
	ZERO(T0) \
	ZERO(T1) \
	SDOT(4, 10, T0) \
	SDOT(5, 11, T0) \
	SDOT(6, 12, T1) \
	SDOT(7, 13, T1) \
	MLAE(S, i0, T0, A) \
	MLAE(S, i1, T1, A)

#define Q4x4CHUNK(S, i0, i1) \
	VLD1.P 32(R14), [V8.B16, V9.B16] \
	VAND V31.B16, V8.B16, V10.B16 \
	VAND V31.B16, V9.B16, V11.B16 \
	VUSHR $4, V8.B16, V12.B16 \
	VUSHR $4, V9.B16, V13.B16 \
	Q4x4COL(R1, 16, S, i0, i1, 14, 15) \
	Q4x4COL(R16, 17, S, i0, i1, 26, 27) \
	Q4x4COL(R17, 18, S, i0, i1, 28, 29) \
	Q4x4COL(R19, 19, S, i0, i1, 20, 21)

#define Q4x4MINS(PTR, M) \
	VLD1.P 32(PTR), [V4.H8, V5.H8] \
	ADDP8H(5, 4, 6) \
	SMULL4H(1, 6, M) \
	SMLAL8H(1, 6, M)

// func dotQ4KQ8Kx4Asm(x, y unsafe.Pointer, stride, nb int, out *float32)
TEXT ·dotQ4KQ8Kx4Asm(SB), NOSPLIT, $0-40
	MOVD x+0(FP), R0
	MOVD y+8(FP), R1
	MOVD stride+16(FP), R3
	MOVD nb+24(FP), R2
	MOVD out+32(FP), R20
	ADD  R3, R1, R16
	ADD  R3, R16, R17
	ADD  R3, R17, R19
	MOVW $0x0f0f0f0f, R3
	VDUP R3, V31.S4
	VMOVI $0, V30.B16
	CBZ  R2, q4x4done

q4x4loop:
	MOVHU (R0), R5
	MOVHU 2(R0), R6
	FMOVS R5, F28
	FCVTHS F28, F28
	FMOVS R6, F29
	FCVTHS F29, F29
	MOVWU.P 4(R1), R7
	MOVWU.P 4(R16), R8
	MOVWU.P 4(R17), R9
	MOVWU.P 4(R19), R10
	VMOV R7, V0.S[0]
	VMOV R8, V0.S[1]
	VMOV R9, V0.S[2]
	VMOV R10, V0.S[3]
	FMULE0(28, 0, 24)             // V24 = d_w * y.d (per column)
	FMULE0(29, 0, 25)             // V25 = dmin_w * y.d

	MOVWU 4(R0), R5
	MOVWU 8(R0), R6
	MOVWU 12(R0), R7
	LSRW $4, R7, R8
	ANDW $0x0f0f0f0f, R8, R8
	LSRW $6, R6, R9
	ANDW $0x03030303, R9, R9
	LSLW $4, R9, R9
	ORRW R9, R8, R8
	ANDW $0x3f3f3f3f, R6, R10
	ANDW $0x0f0f0f0f, R7, R11
	LSRW $6, R5, R9
	ANDW $0x03030303, R9, R9
	LSLW $4, R9, R9
	ORRW R9, R11, R11
	ANDW $0x3f3f3f3f, R5, R12
	ORR  R11<<32, R12, R12
	ORR  R8<<32, R10, R10
	VMOV R12, V2.D[0]
	VMOV R10, V1.D[0]
	UXTL8B(2, 2)
	UXTL8B(1, 1)
	UXTL2_8H(2, 3)
	UXTL4H(2, 2)

	VMOVI $0, V16.B16
	VMOVI $0, V17.B16
	VMOVI $0, V18.B16
	VMOVI $0, V19.B16
	ADD  $16, R0, R14

	Q4x4CHUNK(2, 0, 1)
	Q4x4CHUNK(2, 2, 3)
	Q4x4CHUNK(3, 0, 1)
	Q4x4CHUNK(3, 2, 3)

	Q4x4MINS(R1, 20)
	Q4x4MINS(R16, 21)
	Q4x4MINS(R17, 22)
	Q4x4MINS(R19, 23)

	// lane c of the reduced vectors = column c
	ADDP4S(17, 16, 16)
	ADDP4S(19, 18, 18)
	ADDP4S(18, 16, 16)
	ADDP4S(21, 20, 20)
	ADDP4S(23, 22, 22)
	ADDP4S(22, 20, 20)
	SCVTF4S(16, 16)
	SCVTF4S(20, 20)
	VFMLA V24.S4, V16.S4, V30.S4
	VFMLS V25.S4, V20.S4, V30.S4

	ADD  $144, R0
	SUBS $1, R2
	BNE  q4x4loop

q4x4done:
	VST1 [V30.S4], (R20)
	RET

#define Q6x4COL(PTR, A, sc) \
	VLD1.P 64(PTR), [V20.B16, V21.B16, V22.B16, V23.B16] \
	VMOVI $0, V14.B16 \
	VMOVI $0, V15.B16 \
	VMOVI $0, V17.B16 \
	VMOVI $0, V18.B16 \
	SDOT(20, 24, 14) \
	SDOT(21, 25, 15) \
	SDOT(22, 26, 17) \
	SDOT(23, 27, 18) \
	MLAE(sc, 0, 14, A) \
	MLAE(sc, 1, 15, A) \
	MLAE(sc, 2, 17, A) \
	MLAE(sc, 3, 18, A)

#define Q6x4COMBINE \
	VORR V14.B16, V24.B16, V24.B16 \
	VORR V15.B16, V25.B16, V25.B16 \
	VORR V17.B16, V26.B16, V26.B16 \
	VORR V18.B16, V27.B16, V27.B16 \
	VSUB V30.B16, V24.B16, V24.B16 \
	VSUB V30.B16, V25.B16, V25.B16 \
	VSUB V30.B16, V26.B16, V26.B16 \
	VSUB V30.B16, V27.B16, V27.B16

#define Q6x4COLS(sc) \
	Q6x4COL(R1, 16, sc) \
	Q6x4COL(R16, 19, sc) \
	Q6x4COL(R17, 2, sc) \
	Q6x4COL(R19, 7, sc)

#define Q6x4HALF(scA, scB) \
	VLD1.P 64(R4), [V8.B16, V9.B16, V10.B16, V11.B16] \
	VLD1.P 32(R5), [V12.B16, V13.B16] \
	VSHL $4, V12.B16, V14.B16 \
	VAND V29.B16, V14.B16, V14.B16 \
	VSHL $4, V13.B16, V15.B16 \
	VAND V29.B16, V15.B16, V15.B16 \
	VSHL $2, V12.B16, V17.B16 \
	VAND V29.B16, V17.B16, V17.B16 \
	VSHL $2, V13.B16, V18.B16 \
	VAND V29.B16, V18.B16, V18.B16 \
	VAND V28.B16, V8.B16, V24.B16 \
	VAND V28.B16, V9.B16, V25.B16 \
	VAND V28.B16, V10.B16, V26.B16 \
	VAND V28.B16, V11.B16, V27.B16 \
	Q6x4COMBINE \
	Q6x4COLS(scA) \
	VAND V29.B16, V12.B16, V14.B16 \
	VAND V29.B16, V13.B16, V15.B16 \
	VUSHR $2, V12.B16, V17.B16 \
	VAND V29.B16, V17.B16, V17.B16 \
	VUSHR $2, V13.B16, V18.B16 \
	VAND V29.B16, V18.B16, V18.B16 \
	VUSHR $4, V8.B16, V24.B16 \
	VUSHR $4, V9.B16, V25.B16 \
	VUSHR $4, V10.B16, V26.B16 \
	VUSHR $4, V11.B16, V27.B16 \
	Q6x4COMBINE \
	Q6x4COLS(scB)

// func dotQ6KQ8Kx4Asm(x, y unsafe.Pointer, stride, nb int, out *float32)
TEXT ·dotQ6KQ8Kx4Asm(SB), NOSPLIT, $0-40
	MOVD x+0(FP), R0
	MOVD y+8(FP), R1
	MOVD stride+16(FP), R3
	MOVD nb+24(FP), R2
	MOVD out+32(FP), R20
	ADD  R3, R1, R16
	ADD  R3, R16, R17
	ADD  R3, R17, R19
	MOVW $0x0f0f0f0f, R3
	VDUP R3, V28.S4
	MOVW $0x30303030, R3
	VDUP R3, V29.S4
	MOVW $0x20202020, R3
	VDUP R3, V30.S4
	VMOVI $0, V31.B16
	CBZ  R2, q6x4done

q6x4loop:
	MOVHU 208(R0), R3
	FMOVS R3, F0
	FCVTHS F0, F0
	VDUP V0.S[0], V1.S4           // d_w
	MOVWU.P 4(R1), R7
	MOVWU.P 4(R16), R8
	MOVWU.P 4(R17), R9
	MOVWU.P 4(R19), R10
	VMOV R7, V0.S[0]
	VMOV R8, V0.S[1]
	VMOV R9, V0.S[2]
	VMOV R10, V0.S[3]
	VFMUL4S(0, 1, 1)              // V1 = d_w * y.d per column

	ADD  $192, R0, R3
	VLD1 (R3), [V0.B16]
	SXTL8B(0, 2)
	SXTL2_16B(0, 7)
	SXTL4H(2, 3)
	SXTL2_8H(2, 4)
	SXTL4H(7, 5)
	SXTL2_8H(7, 6)

	MOVD R0, R4
	ADD  $128, R0, R5
	VMOVI $0, V16.B16
	VMOVI $0, V19.B16
	VMOVI $0, V2.B16
	VMOVI $0, V7.B16

	Q6x4HALF(3, 4)
	Q6x4HALF(5, 6)

	ADDP4S(19, 16, 16)
	ADDP4S(7, 2, 2)
	ADDP4S(2, 16, 16)
	SCVTF4S(16, 16)
	VFMLA V1.S4, V16.S4, V31.S4

	ADD  $32, R1                   // skip bsums
	ADD  $32, R16
	ADD  $32, R17
	ADD  $32, R19
	ADD  $210, R0
	SUBS $1, R2
	BNE  q6x4loop

q6x4done:
	VST1 [V31.S4], (R20)
	RET


#define F16x4COL(PTR, A0, A1) \
	VLD1.P 64(PTR), [V8.S4, V9.S4, V10.S4, V11.S4] \
	VFMLA V0.S4, V8.S4, A0.S4 \
	VFMLA V1.S4, V9.S4, A1.S4 \
	VFMLA V2.S4, V10.S4, A0.S4 \
	VFMLA V3.S4, V11.S4, A1.S4

// func dotF16F32x4Asm(x, y unsafe.Pointer, stride, n int, out *float32)
// out[c] = sum_i f16(x[i]) * y[c*stride bytes + 4i]; n multiple of 16.
TEXT ·dotF16F32x4Asm(SB), NOSPLIT, $0-40
	MOVD x+0(FP), R0
	MOVD y+8(FP), R1
	MOVD stride+16(FP), R3
	MOVD n+24(FP), R2
	MOVD out+32(FP), R20
	ADD  R3, R1, R16
	ADD  R3, R16, R17
	ADD  R3, R17, R19
	LSR  $4, R2
	VMOVI $0, V16.B16
	VMOVI $0, V17.B16
	VMOVI $0, V18.B16
	VMOVI $0, V19.B16
	VMOVI $0, V20.B16
	VMOVI $0, V21.B16
	VMOVI $0, V22.B16
	VMOVI $0, V23.B16
	CBZ  R2, f16x4done
f16x4loop:
	VLD1.P 32(R0), [V4.H8, V5.H8]
	FCVTL4H(4, 0)
	FCVTL2_8H(4, 1)
	FCVTL4H(5, 2)
	FCVTL2_8H(5, 3)
	F16x4COL(R1, V16, V17)
	F16x4COL(R16, V18, V19)
	F16x4COL(R17, V20, V21)
	F16x4COL(R19, V22, V23)
	SUBS $1, R2
	BNE  f16x4loop
f16x4done:
	VFADD V17.S4, V16.S4, V16.S4
	VFADD V19.S4, V18.S4, V18.S4
	VFADD V21.S4, V20.S4, V20.S4
	VFADD V23.S4, V22.S4, V22.S4
	FADDP4S(18, 16, 16)
	FADDP4S(22, 20, 20)
	FADDP4S(20, 16, 16)
	VST1 [V16.S4], (R20)
	RET

#define Q80x4COL(off, A) \
	ADD  $(16+32*off), R5, R6 \
	VLD1 (R6), [V4.B16, V5.B16] \
	ZERO(A) \
	SDOT(4, 0, A) \
	SDOT(5, 1, A)

// func dotQ8_0x4Asm(x, y unsafe.Pointer, nb int, out *float32)
// y is the 4-row interleaved activation layout: per block, d[4] f32 then qs[4][32].
TEXT ·dotQ8_0x4Asm(SB), NOSPLIT, $0-32
	MOVD x+0(FP), R0
	MOVD y+8(FP), R5
	MOVD nb+16(FP), R2
	MOVD out+24(FP), R20
	VMOVI $0, V30.B16
	CBZ  R2, q80x4done
q80x4loop:
	MOVHU (R0), R3
	FMOVS R3, F2
	FCVTHS F2, F2
	VLD1 (R5), [V6.S4]            // 4 activation scales
	FMULE0(2, 6, 6)               // V6 = d_w * d_c
	ADD  $2, R0, R4
	VLD1 (R4), [V0.B16, V1.B16]
	Q80x4COL(0, 16)
	Q80x4COL(1, 17)
	Q80x4COL(2, 18)
	Q80x4COL(3, 19)
	ADDP4S(17, 16, 16)
	ADDP4S(19, 18, 18)
	ADDP4S(18, 16, 16)
	SCVTF4S(16, 16)
	VFMLA V6.S4, V16.S4, V30.S4
	ADD  $34, R0
	ADD  $144, R5
	SUBS $1, R2
	BNE  q80x4loop
q80x4done:
	VST1 [V30.S4], (R20)
	RET

// ---- attention kernels for 4 queries at once (generated) ----

#define FMLAE(m, i, n, d) WORD $(0x4F801000 | (((i)&1)<<21) | (((m)&15)<<16) | (((i)>>1)<<11) | ((n)<<5) | (d)) // Vd.4S += Vn.4S * Vm.S[i], m<16

// func scores4Asm(q, kT unsafe.Pointer, dstride, nblk, hd int, out unsafe.Pointer, ostride int)
// q: 4 rows of hd floats (contiguous). kT: [hd][positions] with dstride bytes per d.
// out[i][p] = q_i . kT[:, p] for p < 16*nblk; out rows ostride bytes apart.
TEXT ·scores4Asm(SB), NOSPLIT, $0-56
	MOVD q+0(FP), R0
	MOVD kT+8(FP), R1
	MOVD dstride+16(FP), R2
	MOVD nblk+24(FP), R3
	MOVD hd+32(FP), R4
	MOVD out+40(FP), R5
	MOVD ostride+48(FP), R13
	LSL  $2, R4, R14              // hd*4 bytes
	CBZ  R3, sc4done
sc4blk:
	ZERO(16)
	ZERO(17)
	ZERO(18)
	ZERO(19)
	ZERO(20)
	ZERO(21)
	ZERO(22)
	ZERO(23)
	ZERO(24)
	ZERO(25)
	ZERO(26)
	ZERO(27)
	ZERO(28)
	ZERO(29)
	ZERO(30)
	ZERO(31)
	MOVD R0, R6
	ADD  R14, R6, R10
	ADD  R14, R10, R11
	ADD  R14, R11, R12
	MOVD R1, R7
	LSR  $2, R4, R8
sc4d:
	VLD1.P 16(R6), [V0.S4]
	VLD1.P 16(R10), [V1.S4]
	VLD1.P 16(R11), [V2.S4]
	VLD1.P 16(R12), [V3.S4]
	VLD1 (R7), [V4.S4, V5.S4, V6.S4, V7.S4]
	ADD  R2, R7
	FMLAE(0, 0, 4, 16)
	FMLAE(0, 0, 5, 17)
	FMLAE(0, 0, 6, 18)
	FMLAE(0, 0, 7, 19)
	FMLAE(1, 0, 4, 20)
	FMLAE(1, 0, 5, 21)
	FMLAE(1, 0, 6, 22)
	FMLAE(1, 0, 7, 23)
	FMLAE(2, 0, 4, 24)
	FMLAE(2, 0, 5, 25)
	FMLAE(2, 0, 6, 26)
	FMLAE(2, 0, 7, 27)
	FMLAE(3, 0, 4, 28)
	FMLAE(3, 0, 5, 29)
	FMLAE(3, 0, 6, 30)
	FMLAE(3, 0, 7, 31)
	VLD1 (R7), [V4.S4, V5.S4, V6.S4, V7.S4]
	ADD  R2, R7
	FMLAE(0, 1, 4, 16)
	FMLAE(0, 1, 5, 17)
	FMLAE(0, 1, 6, 18)
	FMLAE(0, 1, 7, 19)
	FMLAE(1, 1, 4, 20)
	FMLAE(1, 1, 5, 21)
	FMLAE(1, 1, 6, 22)
	FMLAE(1, 1, 7, 23)
	FMLAE(2, 1, 4, 24)
	FMLAE(2, 1, 5, 25)
	FMLAE(2, 1, 6, 26)
	FMLAE(2, 1, 7, 27)
	FMLAE(3, 1, 4, 28)
	FMLAE(3, 1, 5, 29)
	FMLAE(3, 1, 6, 30)
	FMLAE(3, 1, 7, 31)
	VLD1 (R7), [V4.S4, V5.S4, V6.S4, V7.S4]
	ADD  R2, R7
	FMLAE(0, 2, 4, 16)
	FMLAE(0, 2, 5, 17)
	FMLAE(0, 2, 6, 18)
	FMLAE(0, 2, 7, 19)
	FMLAE(1, 2, 4, 20)
	FMLAE(1, 2, 5, 21)
	FMLAE(1, 2, 6, 22)
	FMLAE(1, 2, 7, 23)
	FMLAE(2, 2, 4, 24)
	FMLAE(2, 2, 5, 25)
	FMLAE(2, 2, 6, 26)
	FMLAE(2, 2, 7, 27)
	FMLAE(3, 2, 4, 28)
	FMLAE(3, 2, 5, 29)
	FMLAE(3, 2, 6, 30)
	FMLAE(3, 2, 7, 31)
	VLD1 (R7), [V4.S4, V5.S4, V6.S4, V7.S4]
	ADD  R2, R7
	FMLAE(0, 3, 4, 16)
	FMLAE(0, 3, 5, 17)
	FMLAE(0, 3, 6, 18)
	FMLAE(0, 3, 7, 19)
	FMLAE(1, 3, 4, 20)
	FMLAE(1, 3, 5, 21)
	FMLAE(1, 3, 6, 22)
	FMLAE(1, 3, 7, 23)
	FMLAE(2, 3, 4, 24)
	FMLAE(2, 3, 5, 25)
	FMLAE(2, 3, 6, 26)
	FMLAE(2, 3, 7, 27)
	FMLAE(3, 3, 4, 28)
	FMLAE(3, 3, 5, 29)
	FMLAE(3, 3, 6, 30)
	FMLAE(3, 3, 7, 31)
	SUBS $1, R8
	BNE  sc4d
	MOVD R5, R9
	VST1 [V16.S4, V17.S4, V18.S4, V19.S4], (R9)
	ADD  R13, R9
	VST1 [V20.S4, V21.S4, V22.S4, V23.S4], (R9)
	ADD  R13, R9
	VST1 [V24.S4, V25.S4, V26.S4, V27.S4], (R9)
	ADD  R13, R9
	VST1 [V28.S4, V29.S4, V30.S4, V31.S4], (R9)
	ADD  $64, R5
	ADD  $64, R1
	SUBS $1, R3
	BNE  sc4blk
sc4done:
	RET

// func wsum4Asm(p unsafe.Pointer, pstride int, v unsafe.Pointer, vstride, n4, hd int, out unsafe.Pointer, ostride int)
// out[i][0:hd] = sum_{j < 4*n4} p[i][j] * v[j][0:hd]; p rows pstride bytes apart, v rows vstride bytes, hd multiple of 16.
TEXT ·wsum4Asm(SB), NOSPLIT, $0-64
	MOVD p+0(FP), R0
	MOVD pstride+8(FP), R1
	MOVD v+16(FP), R2
	MOVD vstride+24(FP), R3
	MOVD n4+32(FP), R4
	MOVD hd+40(FP), R5
	MOVD out+48(FP), R15
	MOVD ostride+56(FP), R13
	LSR  $4, R5
	CBZ  R5, ws4done
ws4dblk:
	ZERO(16)
	ZERO(17)
	ZERO(18)
	ZERO(19)
	ZERO(20)
	ZERO(21)
	ZERO(22)
	ZERO(23)
	ZERO(24)
	ZERO(25)
	ZERO(26)
	ZERO(27)
	ZERO(28)
	ZERO(29)
	ZERO(30)
	ZERO(31)
	MOVD R0, R6
	ADD  R1, R6, R10
	ADD  R1, R10, R11
	ADD  R1, R11, R12
	MOVD R2, R7
	MOVD R4, R8
	CBZ  R8, ws4store
ws4j:
	VLD1.P 16(R6), [V0.S4]
	VLD1.P 16(R10), [V1.S4]
	VLD1.P 16(R11), [V2.S4]
	VLD1.P 16(R12), [V3.S4]
	VLD1 (R7), [V4.S4, V5.S4, V6.S4, V7.S4]
	ADD  R3, R7
	FMLAE(0, 0, 4, 16)
	FMLAE(0, 0, 5, 17)
	FMLAE(0, 0, 6, 18)
	FMLAE(0, 0, 7, 19)
	FMLAE(1, 0, 4, 20)
	FMLAE(1, 0, 5, 21)
	FMLAE(1, 0, 6, 22)
	FMLAE(1, 0, 7, 23)
	FMLAE(2, 0, 4, 24)
	FMLAE(2, 0, 5, 25)
	FMLAE(2, 0, 6, 26)
	FMLAE(2, 0, 7, 27)
	FMLAE(3, 0, 4, 28)
	FMLAE(3, 0, 5, 29)
	FMLAE(3, 0, 6, 30)
	FMLAE(3, 0, 7, 31)
	VLD1 (R7), [V4.S4, V5.S4, V6.S4, V7.S4]
	ADD  R3, R7
	FMLAE(0, 1, 4, 16)
	FMLAE(0, 1, 5, 17)
	FMLAE(0, 1, 6, 18)
	FMLAE(0, 1, 7, 19)
	FMLAE(1, 1, 4, 20)
	FMLAE(1, 1, 5, 21)
	FMLAE(1, 1, 6, 22)
	FMLAE(1, 1, 7, 23)
	FMLAE(2, 1, 4, 24)
	FMLAE(2, 1, 5, 25)
	FMLAE(2, 1, 6, 26)
	FMLAE(2, 1, 7, 27)
	FMLAE(3, 1, 4, 28)
	FMLAE(3, 1, 5, 29)
	FMLAE(3, 1, 6, 30)
	FMLAE(3, 1, 7, 31)
	VLD1 (R7), [V4.S4, V5.S4, V6.S4, V7.S4]
	ADD  R3, R7
	FMLAE(0, 2, 4, 16)
	FMLAE(0, 2, 5, 17)
	FMLAE(0, 2, 6, 18)
	FMLAE(0, 2, 7, 19)
	FMLAE(1, 2, 4, 20)
	FMLAE(1, 2, 5, 21)
	FMLAE(1, 2, 6, 22)
	FMLAE(1, 2, 7, 23)
	FMLAE(2, 2, 4, 24)
	FMLAE(2, 2, 5, 25)
	FMLAE(2, 2, 6, 26)
	FMLAE(2, 2, 7, 27)
	FMLAE(3, 2, 4, 28)
	FMLAE(3, 2, 5, 29)
	FMLAE(3, 2, 6, 30)
	FMLAE(3, 2, 7, 31)
	VLD1 (R7), [V4.S4, V5.S4, V6.S4, V7.S4]
	ADD  R3, R7
	FMLAE(0, 3, 4, 16)
	FMLAE(0, 3, 5, 17)
	FMLAE(0, 3, 6, 18)
	FMLAE(0, 3, 7, 19)
	FMLAE(1, 3, 4, 20)
	FMLAE(1, 3, 5, 21)
	FMLAE(1, 3, 6, 22)
	FMLAE(1, 3, 7, 23)
	FMLAE(2, 3, 4, 24)
	FMLAE(2, 3, 5, 25)
	FMLAE(2, 3, 6, 26)
	FMLAE(2, 3, 7, 27)
	FMLAE(3, 3, 4, 28)
	FMLAE(3, 3, 5, 29)
	FMLAE(3, 3, 6, 30)
	FMLAE(3, 3, 7, 31)
	SUBS $1, R8
	BNE  ws4j
ws4store:
	MOVD R15, R9
	VST1 [V16.S4, V17.S4, V18.S4, V19.S4], (R9)
	ADD  R13, R9
	VST1 [V20.S4, V21.S4, V22.S4, V23.S4], (R9)
	ADD  R13, R9
	VST1 [V24.S4, V25.S4, V26.S4, V27.S4], (R9)
	ADD  R13, R9
	VST1 [V28.S4, V29.S4, V30.S4, V31.S4], (R9)
	ADD  $64, R15
	ADD  $64, R2
	SUBS $1, R5
	BNE  ws4dblk
ws4done:
	RET
