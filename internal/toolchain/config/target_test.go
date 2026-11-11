package config_test

import (
	"testing"

	"kigumi/internal/lang/target"
	buildconfig "kigumi/internal/toolchain/config"
)

// TestParseTargetUnverifiedABI checks that an unrecognized ABI suffix is marked Unverified rather than rejected outright or silently accepted.
func TestParseTargetUnverifiedABI(t *testing.T) {
	t.Parallel()
	got, err := buildconfig.ParseTarget(buildconfig.CompilerOptions{}, "x86_64-linux-nonsense", "")
	if err != nil {
		t.Fatalf("unknown ABI must build, not error: %v", err)
	}
	if !got.Unverified() {
		t.Fatal("x86_64-linux-nonsense must be marked unverified")
	}
}

// TestParseTargetKnownABI checks that recognized ABI spellings stay verified, so a normal build never gets an unexpected release-policy violation.
func TestParseTargetKnownABI(t *testing.T) {
	t.Parallel()
	for _, triple := range []string{"x86-linux-musl", "aarch64-linux-gnu", "x86_64-linux", "x86_64-windows-msvc"} {
		got, err := buildconfig.ParseTarget(buildconfig.CompilerOptions{}, triple, "")
		if err != nil {
			t.Fatalf("%s: %v", triple, err)
		}
		if got.Unverified() {
			t.Fatalf("%s must be a recognized target, not unverified", triple)
		}
	}
}

// TestTargetLayout reads the C layout facts off a parsed target: the
// pointer width decides usize, the OS decides LLP64 and the calling
// convention, and the triple's ABI segment picks the C library variant.
func TestTargetLayout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		triple            string
		ptrBits           int
		longBits          int
		bigEndian         bool
		charUnsigned      bool
		convention        string
		wordExtends       bool
		aggregateRegBytes int
	}{
		{"x86_64-linux", 64, 64, false, false, "sysv", false, 16},
		{"x86-linux-musl", 32, 32, false, false, "sysv", false, 8},
		{"aarch64-linux", 64, 64, false, true, "aapcs64", false, 16},
		{"arm-linux-gnueabihf", 32, 32, false, true, "aapcs32", false, 8},
		{"riscv64-linux", 64, 64, false, true, "riscv64", true, 16},
		{"wasm32-wasi", 32, 32, false, false, "wasm", false, 0},
		{"x86_64-windows", 64, 32, false, false, "win64", false, 8},
		{"x86_64-uefi", 64, 32, false, false, "win64", false, 8},
		{"aarch64-linux", 64, 64, false, true, "aapcs64", false, 16},
	}
	for _, c := range cases {
		tg, err := buildconfig.ParseTarget(buildconfig.CompilerOptions{}, c.triple, "")
		if err != nil {
			t.Fatalf("%s: %v", c.triple, err)
		}
		l := tg.Layout()
		if l.PtrBits != c.ptrBits || l.C.LongBits != c.longBits || l.BigEndian != c.bigEndian {
			t.Errorf("%s: ptr/long/big = %d/%d/%v, want %d/%d/%v", c.triple, l.PtrBits, l.C.LongBits, l.BigEndian, c.ptrBits, c.longBits, c.bigEndian)
		}
		if l.C.CharUnsigned != c.charUnsigned {
			t.Errorf("%s: CharUnsigned = %v, want %v", c.triple, l.C.CharUnsigned, c.charUnsigned)
		}
		if l.ABI.Convention != c.convention || l.ABI.WordExtends != c.wordExtends {
			t.Errorf("%s: convention/word = %q/%v, want %q/%v", c.triple, l.ABI.Convention, l.ABI.WordExtends, c.convention, c.wordExtends)
		}
		if l.ABI.AggregateRegBytes != c.aggregateRegBytes {
			t.Errorf("%s: aggregate register bytes = %d, want %d", c.triple, l.ABI.AggregateRegBytes, c.aggregateRegBytes)
		}
	}
}

// TestABISegmentReachesLayout checks that the third triple component picks
// the C library variant: msvc's long double is a plain double, mingw's is
// the x87 one.
func TestABISegmentReachesLayout(t *testing.T) {
	t.Parallel()
	gnu, err := buildconfig.ParseTarget(buildconfig.CompilerOptions{}, "x86_64-windows-gnu", "")
	if err != nil {
		t.Fatal(err)
	}
	msvc, err := buildconfig.ParseTarget(buildconfig.CompilerOptions{}, "x86_64-windows-msvc", "")
	if err != nil {
		t.Fatal(err)
	}
	if gnu.ABISegment() != "gnu" || msvc.ABISegment() != "msvc" {
		t.Fatalf("ABISegment = %q and %q, want gnu and msvc", gnu.ABISegment(), msvc.ABISegment())
	}
	if got := gnu.Layout().CSize(target.CLongDouble); got != 16 {
		t.Errorf("mingw long double = %d bytes, want 16", got)
	}
	if got := msvc.Layout().CSize(target.CLongDouble); got != 8 {
		t.Errorf("msvc long double = %d bytes, want 8", got)
	}
}
