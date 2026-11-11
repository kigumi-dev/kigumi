package cheader

import (
	"strings"

	"kigumi/internal/lang/target"
)

// ABI is the handful of C ABI facts that vary by target and that this
// importer's fixed scalar table depends on: everything else in a C ABI
// (calling convention, struct layout, alignment) is already handled by
// zig translate-c itself.
type ABI struct {
	// LLP64 is true on Windows, where c_long/c_ulong are 32-bit unlike
	// every LP64 target this importer otherwise sees.
	LLP64 bool
	// Long32 is true wherever C's long is 32 bits: LLP64 and every ILP32
	// target (386, arm, wasm32), but not the LP64 hosts.
	Long32 bool
	// CharUnsigned is true where plain `char` is unsigned: the Arm ABIs
	// outside Apple and Windows, RISC-V, s390x and PowerPC.
	CharUnsigned bool
	// Int128 is true where C has __int128, which a record field, a
	// parameter and a result may all carry by value.
	Int128 bool
}

// ABIFor is ABIForLayout for an os/arch pair spelled the way
// internal/toolchain/config.Target uses (amd64, arm64, riscv64, 386, arm;
// linux, darwin, windows, ...).
func ABIFor(os, arch string) ABI { return ABIForLayout(target.ForArch(os, arch)) }

// ABIForLayout reads the importer's facts off the target's C model: the
// width of `long`, plain char's signedness and whether __int128 exists.
// Int128 stays false on a 32-bit target even where the compiler has the
// type (wasm32 does), because the checker rejects a 128-bit value there, so
// importing one would only turn a valid header into a semantic error.
func ABIForLayout(l target.Layout) ABI {
	return ABI{
		LLP64:        l.PtrBits == 64 && l.C.LongBits == 32,
		Long32:       l.C.LongBits == 32,
		CharUnsigned: l.C.CharUnsigned,
		Int128:       l.C.Int128 && l.PtrBits >= 64,
	}
}

// mapScalar maps one zig translate-c scalar type keyword to the Kigumi
// ABI-safe type it stands for on this target; ok is false for a
// keyword this importer does not carry across (c_longdouble, and anything
// not in this fixed table).
func mapScalar(word string, abi ABI) (string, bool) {
	switch word {
	case "c_int":
		return "i32", true
	case "c_uint":
		return "u32", true
	case "c_short":
		return "i16", true
	case "c_ushort":
		return "u16", true
	case "c_long":
		if abi.Long32 {
			return "i32", true
		}
		return "i64", true
	case "c_ulong":
		if abi.Long32 {
			return "u32", true
		}
		return "u64", true
	case "c_longlong":
		return "i64", true
	case "c_ulonglong":
		return "u64", true
	case "c_char":
		if abi.CharUnsigned {
			return "u8", true
		}
		return "i8", true
	case "c_schar":
		return "i8", true
	case "c_uchar":
		return "u8", true
	// zig's own fixed-width integer names (u8, i32, ...) already spell the
	// same widths Kigumi uses, and translate-c emits them directly for a
	// stdint.h type and for plain `char` (u8); pass them through unchanged.
	case "u8", "i8", "u16", "i16", "u32", "i32", "u64", "i64":
		return word, true
	case "usize":
		return "usize", true
	case "isize", "c_ptrdiff_t":
		return "isize", true
	case "i128", "u128":
		// A 32-bit target's C compiler has no __int128 to match.
		return word, abi.Int128
	case "f32":
		return "f32", true
	case "f64":
		return "f64", true
	case "bool":
		// Bool is not ABI-safe; a byte is the honest carrier for
		// C's `_Bool` (0/1), chosen here rather than inventing ABI status
		// for the language's Bool.
		return "u8", true
	case "void":
		return "Void", true
	case "noreturn":
		// zig translate-c spells a `_Noreturn`/`__attribute__((noreturn))`
		// function's result this way, the same role Kigumi's Never plays.
		return "Never", true
	}
	return "", false
}

// plainCharSigned reports whether C's plain `char` is signed on this target.
// zig translate-c always spells a plain `char` as `u8`, so on a signed-char
// target the importer has to recover the signedness from the C source.
func (a ABI) plainCharSigned() bool { return !a.CharUnsigned }

// stripTagPrefix drops the struct_/union_/enum_ tag prefix zig gives an
// untypedef'd C tag, so `struct Point` and `typedef struct Point Point`
// import under the same Kigumi name.
func stripTagPrefix(name string) string {
	for _, p := range []string{"struct_", "union_", "enum_"} {
		if s, ok := strings.CutPrefix(name, p); ok {
			return s
		}
	}
	return name
}
