package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// i128 and u128 cross the C ABI by value in the target's own __int128
// registers: parameters, results, a function pointer, a variadic tail, a
// record beside a scalar, and export(C) called back from C.
func TestAotInt128CAbi(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	switch arch := buildconfig.HostTarget().Arch; arch {
	case "amd64", "arm64", "riscv64":
	default:
		t.Skipf("%s's C has no __int128", arch)
	}
	root := t.TempDir()
	src := `extern(C) {
    fn c_sum(a: i128, b: u128) -> i128
    fn c_rot(a: u128, n: i32) -> u128
    fn c_neg(a: i128) -> i128
    fn c_sign(a: i128) -> i32
    fn c_apply(f: extern(C) fn(u128, i128) -> u128, a: u128, b: i128) -> u128
    fn c_pair_sum(p: Pair, x: i128) -> i128
    fn c_pair_flip(p: Pair) -> Pair
    fn c_var(n: i32, ...) -> i128
    fn c_call_back(f: extern(C) fn(i128, u128) -> i128, a: i128, b: u128) -> i128
}

pub type Pair layout(C) = {
    pub a i128
    pub b u128
}

export(C) {
    pub fn kg_mix(a: u128, b: i128) -> u128 {
        a.wrappingAdd(b.toU128() || 0)
    }
    pub fn kg_echo(a: i128, b: u128) -> i128 {
        a.wrappingAdd(b.toI128() || 0)
    }
}

let max: u128 = 340282366920938463463374607431768211455
let min: i128 = -170141183460469231731687303715884105728
let pair = Pair { a: min, b: max }
print "${unsafe { c_sum(min, max) }} ${unsafe { c_neg(min + 1) }} ${unsafe { c_sign(max.toI128() || 0) }}"
print "${unsafe { c_rot(1, 64) }} ${unsafe { c_rot(1, 127) }} ${unsafe { c_rot(max, 1) }}"
print "${unsafe { c_apply(kg_mix, 7, -3) }} ${unsafe { c_call_back(kg_echo, -5, 5) }}"
let g: extern(C) fn(u128, i128) -> u128 = kg_mix
print "${unsafe { g(9, -2) }}"
print "${unsafe { c_pair_sum(pair, 3) }}"
let flipped = unsafe { c_pair_flip(pair) }
print "${flipped.a} ${flipped.b}"
let wide: i128 = 1267650600228229401496703205376
let wideNeg: i128 = -1267650600228229401496703205376
print "${unsafe { c_var(5, wide, wideNeg) }}"
`
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"int128-abi\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	helper := filepath.Join(root, "helper.c")
	cHelper := `#include <stdarg.h>
typedef __int128 i128;
typedef unsigned __int128 u128;
struct pair { i128 a; u128 b; };
i128 c_sum(i128 a, u128 b) { return a + (i128)b; }
u128 c_rot(u128 a, int n) { return (a << n) | (a >> (128 - n)); }
i128 c_neg(i128 a) { return -a; }
int c_sign(i128 a) { return a < 0 ? -1 : (a > 0); }
u128 c_apply(u128 (*f)(u128, i128), u128 a, i128 b) { return f(a, b); }
i128 c_pair_sum(struct pair p, i128 x) { return p.a + (i128)p.b + x; }
struct pair c_pair_flip(struct pair p) { struct pair q; q.a = (i128)p.b; q.b = (u128)p.a; return q; }
i128 c_var(int n, ...) {
    va_list ap;
    va_start(ap, n);
    i128 a = va_arg(ap, i128);
    i128 b = va_arg(ap, i128);
    va_end(ap);
    return a + b + n;
}
i128 c_call_back(i128 (*f)(i128, u128), i128 a, u128 b) { return f(a, b); }
`
	os.WriteFile(helper, []byte(cHelper), 0o644)
	t.Setenv("KIGUMI_CFLAGS", helper)
	std, err := filepath.Abs("../../../std")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	if d := m.Diagnostics(); d != "" {
		t.Fatalf("unexpected diagnostics:\n%s", d)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	want := "170141183460469231731687303715884105727 170141183460469231731687303715884105727 0\n" +
		"18446744073709551616 170141183460469231731687303715884105728 340282366920938463463374607431768211455\n" +
		"7 0\n" +
		"9\n" +
		"-170141183460469231731687303715884105726\n" +
		"-1 170141183460469231731687303715884105728\n" +
		"5\n"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}
