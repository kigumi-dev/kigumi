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

// TestTargetFromTripleQuadruple pins the one constructor's output for every
// supported architecture, OS and ABI family: Triple, OS, Arch, Sys and Tier
// come from the same reading, and each method that reads the triple agrees
// with the fields the constructor derived from it.
func TestTargetFromTripleQuadruple(t *testing.T) {
	t.Parallel()
	cases := []struct {
		triple, os, arch, abi, sys, compile string
		unverified                          bool
	}{
		{"x86_64-linux", "linux", "amd64", "", "posix", "x86_64-linux", false},
		{"x86-linux-musl", "linux", "386", "musl", "posix", "x86-linux-musl", false},
		{"aarch64-linux-gnu", "linux", "arm64", "gnu", "posix", "aarch64-linux-gnu", false},
		{"riscv64-linux", "linux", "riscv64", "", "posix", "riscv64-linux", false},
		{"wasm32-wasi", "wasi", "wasm32", "", "posix", "wasm32-wasi", false},
		{"x86_64-macos", "darwin", "amd64", "", "posix", "x86_64-macos", false},
		{"aarch64-macos", "darwin", "arm64", "", "posix", "aarch64-macos", false},
		{"x86_64-windows", "windows", "amd64", "", "posix", "x86_64-windows", false},
		{"x86_64-windows-gnu", "windows", "amd64", "gnu", "posix", "x86_64-windows-gnu", false},
		{"x86_64-windows-msvc", "windows", "amd64", "msvc", "posix", "x86_64-windows-msvc", false},
		{"x86_64-freebsd", "freebsd", "amd64", "", "posix", "x86_64-freebsd", false},
		{"x86_64-netbsd", "netbsd", "amd64", "", "posix", "x86_64-netbsd", false},
		{"aarch64-openbsd", "openbsd", "arm64", "", "posix", "aarch64-openbsd", false},
		{"x86_64-freestanding", "freestanding", "amd64", "", "none", "x86_64-freestanding", false},
		{"x86_64-none", "freestanding", "amd64", "", "none", "x86_64-freestanding", false},
		{"aarch64-uefi", "uefi", "arm64", "", "none", "aarch64-uefi", false},
		{"arm-uefi", "uefi", "arm", "", "none", "thumb-uefi", false},
		{"arm-linux-musleabihf", "linux", "arm", "musleabihf", "posix", "arm-linux-musleabihf", true},
		{"x86_64-linux-nonsense", "linux", "amd64", "nonsense", "posix", "x86_64-linux-nonsense", true},
	}
	for _, c := range cases {
		got, err := buildconfig.TargetFromTriple(c.triple, "")
		if err != nil {
			t.Fatalf("%s: %v", c.triple, err)
		}
		if got.Triple != c.triple || got.OS != c.os || got.Arch != c.arch || got.Sys != c.sys {
			t.Errorf("%s: quadruple = %q/%q/%q/%q, want %q/%q/%q/%q", c.triple, got.Triple, got.OS, got.Arch, got.Sys, c.triple, c.os, c.arch, c.sys)
		}
		if got.ABISegment() != c.abi {
			t.Errorf("%s: ABISegment = %q, want %q", c.triple, got.ABISegment(), c.abi)
		}
		if got.Unverified() != c.unverified {
			t.Errorf("%s: Unverified = %v, want %v", c.triple, got.Unverified(), c.unverified)
		}
		if got.CompileTriple() != c.compile {
			t.Errorf("%s: CompileTriple = %q, want %q", c.triple, got.CompileTriple(), c.compile)
		}
		if l := got.Layout(); l.OS != c.os || l.Arch != c.arch {
			t.Errorf("%s: Layout = %s/%s, want %s/%s", c.triple, l.OS, l.Arch, c.os, c.arch)
		}
		// A target read back from its own triple must be the same target:
		// that equality is what keeps the four fields from drifting.
		again, err := buildconfig.ParseTarget(buildconfig.CompilerOptions{}, got.Triple, got.Sys)
		if err != nil {
			t.Fatalf("%s: reparse: %v", c.triple, err)
		}
		if again != got {
			t.Errorf("%s: reparse drifted: got %+v, want %+v", c.triple, again, got)
		}
	}
}

// TestParseTargetHostDefault keeps the host path explicit: no triple means
// HostTarget's own facts, and --sys only moves the layer.
func TestParseTargetHostDefault(t *testing.T) {
	t.Parallel()
	host := buildconfig.HostTarget()
	if host.Triple != "" || host.OS == "" || host.Arch == "" || host.Sys != "posix" {
		t.Fatalf("HostTarget = %+v, want the host's OS/Arch with an empty triple and posix", host)
	}
	got, err := buildconfig.ParseTarget(buildconfig.CompilerOptions{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != host {
		t.Fatalf("ParseTarget(\"\", \"\") = %+v, want HostTarget %+v", got, host)
	}
	bare, err := buildconfig.ParseTarget(buildconfig.CompilerOptions{}, "", "none")
	if err != nil {
		t.Fatal(err)
	}
	if bare.OS != host.OS || bare.Arch != host.Arch || bare.Sys != "none" || !bare.Bare() {
		t.Fatalf("--sys none = %+v, want the host OS/Arch with the none layer", bare)
	}
	custom := buildconfig.Target{OS: "darwin", Arch: "arm64", Sys: "posix"}
	got, err = buildconfig.ParseTarget(buildconfig.CompilerOptions{Host: custom}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != custom {
		t.Fatalf("ParseTarget with an explicit host = %+v, want %+v", got, custom)
	}
}

// TestTargetFromTripleSys checks the --sys rules at the constructor, with no
// host involved: none may bare a hosted target, posix contradicts an OS-less
// one, and anything else is rejected.
func TestTargetFromTripleSys(t *testing.T) {
	t.Parallel()
	bare, err := buildconfig.TargetFromTriple("x86_64-linux", "none")
	if err != nil {
		t.Fatalf("hosted triple with --sys none: %v", err)
	}
	if bare.Sys != "none" || !bare.Bare() || bare.OS != "linux" || bare.Arch != "amd64" {
		t.Fatalf("hosted --sys none = %+v, want linux/amd64 with the none layer", bare)
	}
	if _, err := buildconfig.TargetFromTriple("x86_64-freestanding", "posix"); err == nil {
		t.Fatal("--sys posix must contradict a freestanding triple")
	}
	if _, err := buildconfig.TargetFromTriple("aarch64-uefi", "posix"); err == nil {
		t.Fatal("--sys posix must contradict a UEFI triple")
	}
	if _, err := buildconfig.TargetFromTriple("x86_64-linux", "layered"); err == nil {
		t.Fatal("an unknown --sys must be rejected")
	}
	if _, err := buildconfig.TargetFromTriple("", "posix"); err == nil {
		t.Fatal("the empty triple is the host marker, not a triple TargetFromTriple can mint")
	}
	if _, err := buildconfig.TargetFromTriple("sparc-linux", ""); err == nil {
		t.Fatal("an unknown architecture must be rejected")
	}
	if _, err := buildconfig.TargetFromTriple("x86_64-plan9", ""); err == nil {
		t.Fatal("an unknown OS must be rejected")
	}
	if _, err := buildconfig.TargetFromTriple("wasm32-uefi", ""); err == nil {
		t.Fatal("wasm32 must have no UEFI target")
	}
}

// TestABISegmentIsOneFact guards the split that ABISegment and the Tier
// check used to read differently: only an exactly <arch>-<os>-<abi> triple
// names an ABI segment. A longer unverified triple names none, so it cannot
// pick a C library (the Windows long double stays the platform default).
func TestABISegmentIsOneFact(t *testing.T) {
	t.Parallel()
	cases := []struct {
		triple, abi string
		longDouble  int
	}{
		{"x86_64-linux", "", 16},
		{"x86_64-linux-musl", "musl", 16},
		{"x86_64-windows-msvc", "msvc", 8},
		{"x86_64-windows-gnu", "gnu", 16},
		{"x86_64-windows-msvc-extra", "", 16},
	}
	for _, c := range cases {
		got, err := buildconfig.TargetFromTriple(c.triple, "")
		if err != nil {
			t.Fatalf("%s: %v", c.triple, err)
		}
		if got.ABISegment() != c.abi {
			t.Errorf("%s: ABISegment = %q, want %q", c.triple, got.ABISegment(), c.abi)
		}
		if n := got.Layout().CSize(target.CLongDouble); n != c.longDouble {
			t.Errorf("%s: long double = %d bytes, want %d", c.triple, n, c.longDouble)
		}
	}
}

// TestStaticLibc reads the static C library off the ABI segment instead of
// scanning the triple for "-musl", including the arm spelling.
func TestStaticLibc(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"x86_64-linux":            false,
		"x86_64-linux-gnu":        false,
		"x86_64-windows-msvc":     false,
		"x86_64-linux-musl":       true,
		"aarch64-linux-musl":      true,
		"arm-linux-musleabihf":    true,
		"x86_64-linux-musl-extra": false,
	}
	for triple, want := range cases {
		got, err := buildconfig.TargetFromTriple(triple, "")
		if err != nil {
			t.Fatalf("%s: %v", triple, err)
		}
		if got.StaticLibc() != want {
			t.Errorf("%s: StaticLibc = %v, want %v", triple, got.StaticLibc(), want)
		}
	}
	if (buildconfig.Target{}).StaticLibc() {
		t.Error("the host target has no static musl ABI")
	}
}

// TestWithTripleKeepsPolicy retargets a fully configured target: the
// quadruple follows the new triple and the policy fields survive.
func TestWithTripleKeepsPolicy(t *testing.T) {
	t.Parallel()
	target, err := buildconfig.TargetFromTriple("x86_64-linux", "")
	if err != nil {
		t.Fatal(err)
	}
	target.Allocator, target.Profile, target.BlockingExecutor = "none", "recoverable-only", "isr"
	target.StrictStack, target.StackBudgetKiB = true, 64
	next, err := target.WithTriple("aarch64-freestanding")
	if err != nil {
		t.Fatal(err)
	}
	if next.Triple != "aarch64-freestanding" || next.OS != "freestanding" || next.Arch != "arm64" || next.Sys != "none" {
		t.Fatalf("retargeted quadruple = %+v", next)
	}
	if next.Allocator != target.Allocator || next.Profile != target.Profile || next.BlockingExecutor != target.BlockingExecutor || next.StrictStack != target.StrictStack || next.StackBudgetKiB != target.StackBudgetKiB {
		t.Fatalf("retargeting dropped policy: %+v", next)
	}
	if got, err := target.WithTriple("sparc-linux"); err == nil || got != target {
		t.Fatalf("an unparsable triple must leave the target unchanged, got %+v, %v", got, err)
	}
	bare, err := buildconfig.TargetFromTriple("x86_64-linux", "none")
	if err != nil {
		t.Fatal(err)
	}
	if kept, err := bare.WithTriple("aarch64-linux"); err != nil || kept.Sys != "none" {
		t.Fatalf("an explicit --sys none must survive retargeting: %+v, %v", kept, err)
	}
}

// TestTargetName keeps the report label in one place: the user's triple when
// there is one, else the host's platform names.
func TestTargetName(t *testing.T) {
	t.Parallel()
	target, err := buildconfig.TargetFromTriple("x86_64-linux", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := target.Name(); got != "x86_64-linux" {
		t.Errorf("Name = %q, want the triple", got)
	}
	host := buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "posix"}
	if got := host.Name(); got != "linux-amd64" {
		t.Errorf("host Name = %q, want linux-amd64", got)
	}
}
