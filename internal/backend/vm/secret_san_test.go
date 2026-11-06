package vm_test

import (
	"bytes"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
)

// secretSanProgram checks, lowers and plans a main-package source the way the
// VM corpus does, so a SecretSan test runs the same MIR shape as a real run.
func secretSanProgram(t *testing.T, src string) *mir.Program {
	t.Helper()
	res := checkSource(t, src)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	if prog.HasErrors() {
		t.Fatal("ownership errors")
	}
	plans := []mirplan.Transform{
		{Name: "plan-rc", Run: mirplan.RC},
		{Name: "specialize", Run: mirplan.Specialize},
		{Name: "elide-rc", Run: mirplan.ElideRC},
		{Name: "plan-direct", Run: mirplan.PlanDirect},
	}
	if err := mirplan.Apply(prog, plans...); err != nil {
		t.Fatalf("plans: %v", err)
	}
	return prog
}

const secretSanPositiveSrc = `import secret from std/secret

// classify -> ctLt/select -> declassify is the constant-time shape the
// checker must accept: the secret feed only secret primitives, and the
// result is declassified before it is returned.
fn ct_choose(a: u8, b: u8) -> u8 {
    let x = secret.SecretU8.classify(a)
    let y = secret.SecretU8.classify(b)
    let lt = x.ctLt(y)
    let picked = lt.selectU8(x, y)
    (unsafe { picked.declassify() })
}

// A secret value that is not declassified stays labelled.
fn ct_keep(a: u8) -> secret.SecretU8 {
    secret.SecretU8.classify(a)
}

fn main() {
    ct_keep(1)
}
`

// TestSecretSanPositiveConstantTime is the positive control: the CT shape
// passes the sinks and the value declassify returns carries no label.
func TestSecretSanPositiveConstantTime(t *testing.T) {
	prog := secretSanProgram(t, secretSanPositiveSrc)

	var out, errOut bytes.Buffer
	sh, err := vm.SecretSanRun(prog, vm.Options{}, "ct_choose", []int64{5, 3}, []bool{false, false}, &out, &errOut)
	if err != nil {
		t.Fatalf("SecretSanRun: %v\n%s", err, errOut.String())
	}
	if sh.Tainted() {
		t.Errorf("declassified result is still shadowed: %+v", sh)
	}

	sh, err = vm.SecretSanRun(prog, vm.Options{}, "ct_keep", []int64{7}, []bool{false}, &out, &errOut)
	if err != nil {
		t.Fatalf("SecretSanRun(ct_keep): %v\n%s", err, errOut.String())
	}
	if !sh.Tainted() {
		t.Error("classify result carries no shadow")
	}
}

const secretSanBitsSrc = `fn and_zero(s: u64) -> u64 { s & 0 }
fn or_zero(s: u64) -> u64 { s | 0 }
fn and_one(s: u64) -> u64 { s & 1 }
fn xor_all(s: u64) -> u64 { s ^ 0 }
fn shl_three(s: u64) -> u64 { s << 3 }
fn shr_three(s: u64) -> u64 { s >> 3 }
fn not_bits(s: u64) -> u64 { ~s }
fn wrapping_add_zero(s: u64) -> u64 { s.wrappingAdd(0) }
fn main() { and_zero(1) }
`

// TestSecretSanBitRules pins the bit-precise part of the label: AND with a
// public zero clears it, OR with a public zero keeps it, and shifts move the
// bits with the value. Arithmetic is conservative on purpose.
func TestSecretSanBitRules(t *testing.T) {
	prog := secretSanProgram(t, secretSanBitsSrc)
	full := ^uint64(0)
	tests := []struct {
		fn   string
		want vm.ShadowValue
	}{
		{"and_zero", vm.ShadowValue{}},
		{"or_zero", vm.ShadowValue{Lo: full}},
		{"and_one", vm.ShadowValue{Lo: 1}},
		{"xor_all", vm.ShadowValue{Lo: full}},
		{"shl_three", vm.ShadowValue{Lo: full << 3}},
		{"shr_three", vm.ShadowValue{Lo: full >> 3}},
		{"not_bits", vm.ShadowValue{Lo: full}},
		{"wrapping_add_zero", vm.ShadowValue{Lo: full}},
	}
	for _, tc := range tests {
		t.Run(tc.fn, func(t *testing.T) {
			sh, err := vm.SecretSanRun(prog, vm.Options{}, tc.fn, []int64{0x0F0F0F0F0F0F0F0F}, []bool{true}, nil, nil)
			if err != nil {
				t.Fatalf("SecretSanRun: %v", err)
			}
			if sh != tc.want {
				t.Errorf("%s: shadow %+v, want %+v", tc.fn, sh, tc.want)
			}
		})
	}
}

// TestSecretSanSinks is the negative control set: each function feeds a
// secret argument into one of M4's sinks and must fail with the diagnostic
// naming it. The same function before the checker (off by default) succeeds.
func TestSecretSanSinks(t *testing.T) {
	const src = `fn leak_branch(s: u64) -> u64 {
    if s > 3 { 1 } else { 2 }
}

fn leak_match(s: Int) -> Int {
    match s {
        0 -> 1
        _ -> 2
    }
}

fn leak_index(i: usize) -> u64 {
    let xs = Array.of[u64](10, 20, 30, 40)
    xs[i]
}

fn leak_alloc(n: usize) -> usize {
    Bytes.zeros(n).len()
}

fn leak_div(d: u64) -> u64 {
    100 / d
}

fn leak_rem(d: u64) -> u64 {
    100 % d
}

fn leak_overflow(s: u64) -> u64 {
    s + 1
}

fn leak_io(s: u64) {
    print "value=${s}"
}

fn main() {
    leak_branch(1)
    leak_match(1)
    leak_index(0)
    leak_alloc(1)
    leak_div(1)
    leak_rem(1)
    leak_overflow(1)
    leak_io(1)
}
`
	prog := secretSanProgram(t, src)
	tests := []struct {
		name string
		fn   string
		arg  int64
		want string
	}{
		{"branch", "leak_branch", 9, "conditional branch"},
		{"match", "leak_match", 9, "conditional branch"},
		{"index", "leak_index", 2, "an index"},
		{"alloc", "leak_alloc", 16, "allocation size"},
		{"div", "leak_div", 4, "division operand"},
		{"rem", "leak_rem", 4, "division operand"},
		{"overflow", "leak_overflow", 4, "overflow check"},
		{"io", "leak_io", 4, "host I/O"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := vm.SecretSanRun(prog, vm.Options{}, tc.fn, []int64{tc.arg}, []bool{true}, nil, nil)
			if err == nil {
				t.Fatalf("%s: secret input reached a sink without a diagnostic", tc.fn)
			}
			if !strings.Contains(err.Error(), "SecretSan:") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s: diagnostic %q does not name %q", tc.fn, err, tc.want)
			}

			// Default off: the same function runs and reports no SecretSan error.
			if _, err := vm.SecretSanRun(prog, vm.Options{}, tc.fn, []int64{tc.arg}, []bool{false}, nil, nil); err != nil {
				t.Fatalf("%s without a secret input must pass: %v", tc.fn, err)
			}
		})
	}
}

// TestSecretSanOffByDefault runs a leaking function through the ordinary
// entry with SecretSan off: the mode must change nothing.
func TestSecretSanOffByDefault(t *testing.T) {
	prog := secretSanProgram(t, `fn main() {
    let mut x: u64 = 0
    let xs = Array.of[u64](10, 20, 30)
    x = xs[1]
    print "${x}"
}
`)
	var out, errOut bytes.Buffer
	code, err := vm.Run(prog, vm.Options{}, &out, &errOut)
	if err != nil || code != 0 {
		t.Fatalf("vm: %v (exit %d)\n%s", err, code, errOut.String())
	}
	if got := out.String(); got != "20\n" {
		t.Errorf("stdout %q, want %q", got, "20\n")
	}
}

const secretSanTraceSrc = `import secret from std/secret

// The same public input and two different secret inputs: every block,
// index, allocation and RC operation must match.
fn ct_trace(a: u32, b: u32) -> u32 {
    let x = secret.SecretU32.classify(a)
    let y = secret.SecretU32.classify(b)
    let lt = x.ctLt(y)
    (unsafe { lt.selectU32(x, y).declassify() })
}

// A branch on a secret changes the executed blocks.
fn leak_trace_branch(a: u64, b: u64) -> u64 {
    if a < b { 1 } else { 2 }
}

// A secret index leaves the blocks alone but changes an access.
fn leak_trace_index(i: usize) -> u64 {
    let xs = Array.of[u64](10, 20, 30, 40)
    xs[i]
}

// A secret-sized allocation leaves the blocks alone but changes an
// allocation event's size.
fn leak_trace_alloc(n: usize) -> usize {
    Bytes.zeros(n).len()
}

fn main() {
    ct_trace(1, 2)
}
`

// TestSecretSanTwoRunTrace is the two-run mode: a constant-time function
// traces identically with two secret inputs, and a branch or index that
// depends on one makes the traces differ.
func TestSecretSanTwoRunTrace(t *testing.T) {
	prog := secretSanProgram(t, secretSanTraceSrc)

	if err := vm.CompareTraces(prog, vm.Options{}, "ct_trace", []int64{1, 2}, []int64{1, 3}, nil, nil); err != nil {
		t.Fatalf("constant-time function traced differently: %v", err)
	}

	if err := vm.CompareTraces(prog, vm.Options{}, "leak_trace_branch", []int64{1, 2}, []int64{2, 1}, nil, nil); err == nil {
		t.Fatal("branch on a secret produced equal traces")
	} else if !strings.Contains(err.Error(), "executed blocks") {
		t.Fatalf("branch mismatch reported %q", err)
	}

	if err := vm.CompareTraces(prog, vm.Options{}, "leak_trace_index", []int64{0}, []int64{2}, nil, nil); err == nil {
		t.Fatal("secret index produced equal traces")
	} else if !strings.Contains(err.Error(), "object-relative indices") {
		t.Fatalf("index mismatch reported %q", err)
	}

	if err := vm.CompareTraces(prog, vm.Options{}, "leak_trace_alloc", []int64{1}, []int64{2}, nil, nil); err == nil {
		t.Fatal("secret-sized allocation produced equal traces")
	} else if !strings.Contains(err.Error(), "allocations") {
		t.Fatalf("allocation mismatch reported %q", err)
	}
}
