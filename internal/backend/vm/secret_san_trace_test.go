package vm_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

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

// TestSecretSanTwoRunTrace: a constant-time function traces identically with two secret inputs,
// while a branch, index or allocation that depends on one makes the traces differ.
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
