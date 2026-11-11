package target

import "testing"

// TestLayoutFacts pins the per-target C model and calling convention the
// rest of the compiler reads instead of host assumptions. The values are
// what clang (zig cc) reports for each target: long's width, plain char's
// signedness, long double's size and alignment, and which narrow integers
// the convention extends.
func TestLayoutFacts(t *testing.T) {
	cases := []struct {
		os, arch     string
		ptrBits      int
		bigEndian    bool
		longBits     int
		charUnsigned bool
		int128       bool
		ldSize, ldAl int
		convention   string
		narrow, word bool
		aggBytes     int
	}{
		{"linux", "amd64", 64, false, 64, false, true, 16, 16, "sysv", true, false, 16},
		{"linux", "386", 32, false, 32, false, false, 12, 4, "sysv", true, false, 8},
		{"linux", "arm64", 64, false, 64, true, true, 16, 16, "aapcs64", false, false, 16},
		{"linux", "arm", 32, false, 32, true, false, 8, 8, "aapcs32", true, false, 8},
		{"linux", "riscv64", 64, false, 64, true, true, 16, 16, "riscv64", true, true, 16},
		{"wasi", "wasm32", 32, false, 32, false, true, 16, 16, "wasm", true, false, 0},
		{"linux", "mips", 32, true, 32, false, false, 8, 8, "mips", false, false, 8},
		{"linux", "mipsle", 32, false, 32, false, false, 8, 8, "mips", false, false, 8},
		{"linux", "mips64", 64, true, 64, false, true, 16, 16, "mips", false, false, 8},
		{"linux", "mips64le", 64, false, 64, false, true, 16, 16, "mips", false, false, 8},
		{"linux", "ppc64", 64, true, 64, true, true, 8, 8, "powerpc64", false, false, 16},
		{"linux", "ppc64le", 64, false, 64, true, true, 8, 8, "powerpc64", false, false, 16},
		{"linux", "s390x", 64, true, 64, true, true, 16, 8, "s390x", false, false, 16},
		{"linux", "loong64", 64, false, 64, false, true, 16, 16, "loongarch64", false, false, 16},
		// The OS changes LLP64, char signedness and the Microsoft conventions.
		{"windows", "amd64", 64, false, 32, false, true, 16, 16, "win64", false, false, 8},
		{"windows", "arm64", 64, false, 32, false, true, 8, 8, "aapcs64", false, false, 16},
		{"windows", "386", 32, false, 32, false, false, 12, 4, "sysv", true, false, 8},
		{"uefi", "amd64", 64, false, 32, false, true, 8, 8, "win64", false, false, 8},
		{"uefi", "386", 32, false, 32, false, false, 8, 8, "sysv", true, false, 8},
		{"uefi", "arm", 32, false, 32, false, false, 8, 8, "aapcs32", true, false, 8},
		{"darwin", "arm64", 64, false, 64, false, true, 8, 8, "aapcs64", true, false, 16},
		{"darwin", "amd64", 64, false, 64, false, true, 16, 16, "sysv", true, false, 16},
		{"freebsd", "arm64", 64, false, 64, true, true, 16, 16, "aapcs64", false, false, 16},
		{"freestanding", "amd64", 64, false, 64, false, true, 16, 16, "sysv", true, false, 16},
	}
	for _, c := range cases {
		l := ForArch(c.os, c.arch)
		name := c.os + "/" + c.arch
		if l.PtrBits != c.ptrBits {
			t.Errorf("%s: PtrBits = %d, want %d", name, l.PtrBits, c.ptrBits)
		}
		if l.BigEndian != c.bigEndian {
			t.Errorf("%s: BigEndian = %v, want %v", name, l.BigEndian, c.bigEndian)
		}
		if l.C.LongBits != c.longBits {
			t.Errorf("%s: long = %d bits, want %d", name, l.C.LongBits, c.longBits)
		}
		if l.C.CharUnsigned != c.charUnsigned {
			t.Errorf("%s: CharUnsigned = %v, want %v", name, l.C.CharUnsigned, c.charUnsigned)
		}
		if l.C.Int128 != c.int128 {
			t.Errorf("%s: Int128 = %v, want %v", name, l.C.Int128, c.int128)
		}
		if got := l.CSize(CLongDouble); got != c.ldSize {
			t.Errorf("%s: sizeof(long double) = %d, want %d", name, got, c.ldSize)
		}
		if got := l.CAlign(CLongDouble); got != c.ldAl {
			t.Errorf("%s: _Alignof(long double) = %d, want %d", name, got, c.ldAl)
		}
		if l.ABI.Convention != c.convention {
			t.Errorf("%s: convention = %q, want %q", name, l.ABI.Convention, c.convention)
		}
		if l.ABI.NarrowExtends != c.narrow || l.ABI.WordExtends != c.word {
			t.Errorf("%s: extends narrow/word = %v/%v, want %v/%v", name, l.ABI.NarrowExtends, l.ABI.WordExtends, c.narrow, c.word)
		}
		if l.ABI.AggregateRegBytes != c.aggBytes {
			t.Errorf("%s: aggregate register bytes = %d, want %d", name, l.ABI.AggregateRegBytes, c.aggBytes)
		}
	}
}

// TestCScalarLayout pins the sizes and alignments the C boundary reads:
// 32-bit targets keep a 64-bit long long with a 4-byte alignment, and long
// double is not a function of its width alone.
func TestCScalarLayout(t *testing.T) {
	type scalar struct{ size, align int }
	cases := []struct {
		os, arch string
		want     map[Scalar]scalar
	}{
		{"linux", "amd64", map[Scalar]scalar{
			CChar: {1, 1}, CShort: {2, 2}, CInt: {4, 4}, CLong: {8, 8}, CLongLong: {8, 8},
			CFloat: {4, 4}, CDouble: {8, 8}, CLongDouble: {16, 16}, CPointer: {8, 8}, CInt128: {16, 16},
		}},
		{"linux", "386", map[Scalar]scalar{
			CChar: {1, 1}, CShort: {2, 2}, CInt: {4, 4}, CLong: {4, 4}, CLongLong: {8, 4},
			CFloat: {4, 4}, CDouble: {8, 4}, CLongDouble: {12, 4}, CPointer: {4, 4}, CInt128: {16, 16},
		}},
		{"linux", "arm", map[Scalar]scalar{
			CLong: {4, 4}, CLongLong: {8, 8}, CDouble: {8, 8}, CLongDouble: {8, 8}, CPointer: {4, 4},
		}},
		{"linux", "arm64", map[Scalar]scalar{
			CLong: {8, 8}, CLongLong: {8, 8}, CLongDouble: {16, 16}, CPointer: {8, 8},
		}},
		{"linux", "riscv64", map[Scalar]scalar{
			CLong: {8, 8}, CDouble: {8, 8}, CPointer: {8, 8},
		}},
		{"wasi", "wasm32", map[Scalar]scalar{
			CLong: {4, 4}, CLongLong: {8, 8}, CDouble: {8, 8}, CLongDouble: {16, 16}, CPointer: {4, 4},
		}},
		{"linux", "ppc64", map[Scalar]scalar{
			CLong: {8, 8}, CLongDouble: {8, 8}, CPointer: {8, 8},
		}},
		{"linux", "s390x", map[Scalar]scalar{
			CLong: {8, 8}, CLongDouble: {16, 8}, CPointer: {8, 8},
		}},
		{"uefi", "amd64", map[Scalar]scalar{
			CLong: {4, 4}, CLongDouble: {8, 8}, CPointer: {8, 8},
		}},
		{"darwin", "arm64", map[Scalar]scalar{
			CLong: {8, 8}, CLongDouble: {8, 8}, CPointer: {8, 8},
		}},
	}
	for _, c := range cases {
		l := ForArch(c.os, c.arch)
		for s, want := range c.want {
			if got := l.CSize(s); got != want.size {
				t.Errorf("%s/%s scalar %d: size = %d, want %d", c.os, c.arch, s, got, want.size)
			}
			if got := l.CAlign(s); got != want.align {
				t.Errorf("%s/%s scalar %d: align = %d, want %d", c.os, c.arch, s, got, want.align)
			}
		}
	}
}

// TestForOSABIPicksMicrosoftLongDouble checks that the triple's ABI segment
// reaches the C model: the msvc C library makes long double a double on
// x86, while mingw keeps the x87 one.
func TestForOSABIPicksMicrosoftLongDouble(t *testing.T) {
	gnu := ForOSABI("windows", "amd64", "gnu")
	msvc := ForOSABI("windows", "amd64", "msvc")
	if gnu.CSize(CLongDouble) != 16 || msvc.CSize(CLongDouble) != 8 {
		t.Errorf("long double: gnu %d, msvc %d; want 16 and 8", gnu.CSize(CLongDouble), msvc.CSize(CLongDouble))
	}
	if msvc.ABI.Convention != "win64" || gnu.ABI.Convention != "win64" {
		t.Errorf("both Windows x86-64 ABIs use win64, got %q and %q", gnu.ABI.Convention, msvc.ABI.Convention)
	}
}
