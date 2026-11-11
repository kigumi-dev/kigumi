package target

// Scalar names a C scalar type whose size or alignment a target fixes.
type Scalar uint8

const (
	CChar Scalar = iota
	CShort
	CInt
	CLong
	CLongLong
	CFloat
	CDouble
	CLongDouble
	CPointer
	CInt128
)

// CModel is the target's C scalar model: the sizes of the standard integer
// and floating types, and the two properties the LLVM datalayout does not
// state (plain char's signedness and whether __int128 exists).
type CModel struct {
	ShortBits, IntBits, LongBits, LongLongBits int
	FloatBits, DoubleBits                      int
	// LongDoubleBits and LongDoubleAlign are sizeof and _Alignof of long
	// double in bits and bytes; they are not a function of the bit width
	// (i386's 12-byte long double aligns to 4, and the Microsoft ABIs make
	// long double a plain double).
	LongDoubleBits, LongDoubleAlign int
	// CharUnsigned reports plain `char` being unsigned: the Arm ABIs
	// outside Apple and Microsoft, RISC-V, s390x and PowerPC.
	CharUnsigned bool
	// Int128 reports the compiler having __int128.
	Int128 bool
}

// ABI is the target's C calling convention and the aggregate rule that goes
// with it.
type ABI struct {
	// Convention names the calling convention: "sysv", "win64", "aapcs64",
	// "aapcs32", "riscv64", "wasm", "mips", "powerpc64", "s390x" or
	// "loongarch64".
	Convention string
	// NarrowExtends and WordExtends report which integer parameters and
	// results the convention extends to a full register (LLVM's signext and
	// zeroext): i8 and i16 for NarrowExtends, i32 for WordExtends too.
	NarrowExtends, WordExtends bool
	// AggregateRegBytes is the largest aggregate the convention moves in
	// registers; a larger one travels through memory. 0 means every
	// aggregate travels through memory (wasm32).
	AggregateRegBytes int
}

// model builds a CModel with the widths every supported target shares; the
// caller passes the three that vary.
func model(longBits, longDoubleBits, longDoubleAlign int, charUnsigned, int128 bool) CModel {
	return CModel{
		ShortBits: 16, IntBits: 32, LongBits: longBits, LongLongBits: 64,
		FloatBits: 32, DoubleBits: 64,
		LongDoubleBits: longDoubleBits, LongDoubleAlign: longDoubleAlign,
		CharUnsigned: charUnsigned, Int128: int128,
	}
}

// CSize is the size in bytes of a C scalar on the target.
func (l Layout) CSize(s Scalar) int {
	switch s {
	case CChar:
		return 1
	case CShort:
		return l.C.ShortBits / 8
	case CInt:
		return l.C.IntBits / 8
	case CLong:
		return l.C.LongBits / 8
	case CLongLong:
		return l.C.LongLongBits / 8
	case CFloat:
		return l.C.FloatBits / 8
	case CDouble:
		return l.C.DoubleBits / 8
	case CLongDouble:
		return l.C.LongDoubleBits / 8
	case CPointer:
		return l.PtrBits / 8
	case CInt128:
		return 16
	}
	return 0
}

// CAlign is the alignment in bytes of a C scalar inside a struct on the
// target. It is the C type's own alignment, not the memory ABI alignment a
// datalayout can give a byte type (s390x aligns an i8 load to 2, while
// _Alignof(char) stays 1).
func (l Layout) CAlign(s Scalar) int {
	switch s {
	case CChar:
		return 1
	case CShort:
		return 2
	case CInt, CFloat:
		return 4
	case CLong:
		return l.CSize(CLong)
	case CLongLong:
		// 64-bit integers keep their own ABI alignment: 4 on i386, 8
		// elsewhere.
		return l.ScalarAlign(false, 64)
	case CDouble:
		return l.ScalarAlign(true, 64)
	case CLongDouble:
		return l.C.LongDoubleAlign
	case CPointer:
		return l.PtrBits / 8
	case CInt128:
		return l.ScalarAlign(false, 128)
	}
	return 0
}

// archFacts is what an architecture contributes to a Layout before the OS
// adjusts it. The defaults are the Unix ones (Linux, the BSDs and a
// freestanding build compiled against the same musl headers).
type archFacts struct {
	ptrBits   int
	bigEndian bool
	c         CModel
	abi       ABI
}

// The conventions as verified against clang (zig cc) for each target: which
// narrow integers it extends, and how large an aggregate it still moves in
// registers.
var (
	abiSysV64  = ABI{Convention: "sysv", NarrowExtends: true, AggregateRegBytes: 16}
	abiWin64   = ABI{Convention: "win64", AggregateRegBytes: 8}
	abiSysV32  = ABI{Convention: "sysv", NarrowExtends: true, AggregateRegBytes: 8}
	abiAAPCS64 = ABI{Convention: "aapcs64", AggregateRegBytes: 16}
	abiAAPCS32 = ABI{Convention: "aapcs32", NarrowExtends: true, AggregateRegBytes: 8}
	abiRV64    = ABI{Convention: "riscv64", NarrowExtends: true, WordExtends: true, AggregateRegBytes: 16}
	abiWasm    = ABI{Convention: "wasm", NarrowExtends: true}
	abiMIPS    = ABI{Convention: "mips", AggregateRegBytes: 8}
	abiPPC64   = ABI{Convention: "powerpc64", AggregateRegBytes: 16}
	abiS390X   = ABI{Convention: "s390x", AggregateRegBytes: 16}
	abiLoong64 = ABI{Convention: "loongarch64", AggregateRegBytes: 16}
)

// archFactsTable is the per-architecture part of the layout table. Only its
// pointer width is reachable through ParseTarget today; the rest is what a
// host built for that architecture reports, so a GOARCH outside the
// toolchain's --target set still gets a correct Layout.
var archFactsTable = map[string]archFacts{
	"amd64":    {64, false, model(64, 128, 16, false, true), abiSysV64},
	"386":      {32, false, model(32, 96, 4, false, false), abiSysV32},
	"arm64":    {64, false, model(64, 128, 16, true, true), abiAAPCS64},
	"arm":      {32, false, model(32, 64, 8, true, false), abiAAPCS32},
	"riscv64":  {64, false, model(64, 128, 16, true, true), abiRV64},
	"wasm32":   {32, false, model(32, 128, 16, false, true), abiWasm},
	"mips":     {32, true, model(32, 64, 8, false, false), abiMIPS},
	"mipsle":   {32, false, model(32, 64, 8, false, false), abiMIPS},
	"mips64":   {64, true, model(64, 128, 16, false, true), abiMIPS},
	"mips64le": {64, false, model(64, 128, 16, false, true), abiMIPS},
	"ppc64":    {64, true, model(64, 64, 8, true, true), abiPPC64},
	"ppc64le":  {64, false, model(64, 64, 8, true, true), abiPPC64},
	"s390x":    {64, true, model(64, 128, 8, true, true), abiS390X},
	"loong64":  {64, false, model(64, 128, 16, false, true), abiLoong64},
}

// microsoftOS reports an OS whose C ABI is the Microsoft one.
func microsoftOS(os string) bool { return os == "windows" || os == "uefi" }

// applyPlatform applies the OS and C library differences to an
// architecture's Unix defaults: Windows and UEFI are LLP64 with the
// Microsoft long double (UEFI, and Windows' msvc ABI, make long double a
// plain double; mingw keeps the x87 one on x86), and Apple's Arm64 ABI
// differs from AAPCS64 in char signedness, long double and integer
// extension.
func applyPlatform(l *Layout, os, arch, abi string) {
	switch {
	case microsoftOS(os):
		l.C.LongBits = 32
		l.C.CharUnsigned = false
		microsoft := os == "uefi" || abi == "msvc"
		switch {
		case microsoft:
			l.C.LongDoubleBits, l.C.LongDoubleAlign = 64, 8
		case arch == "amd64":
			// mingw on x86-64 keeps the x87 long double.
			l.C.LongDoubleBits, l.C.LongDoubleAlign = 128, 16
		case arch == "386":
			l.C.LongDoubleBits, l.C.LongDoubleAlign = 96, 4
		default:
			l.C.LongDoubleBits, l.C.LongDoubleAlign = 64, 8
		}
		// Both the mingw and the msvc C libraries use the Microsoft
		// calling convention; only long double differs.
		switch arch {
		case "amd64":
			l.ABI = abiWin64
		case "arm64":
			l.ABI = ABI{Convention: "aapcs64", AggregateRegBytes: 16}
		}
	case os == "darwin" && arch == "arm64":
		// Apple's arm64 ABI: signed char, long double is a plain double,
		// and narrow integers extended like x86-64.
		l.C.CharUnsigned = false
		l.C.LongDoubleBits, l.C.LongDoubleAlign = 64, 8
		l.ABI.NarrowExtends = true
	}
}
