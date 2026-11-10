package vm_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

const secretSanIndexSrc = `type Cell = {
    pub mut v u64
}

fn cells() -> Array[Cell] {
    Array.of(Cell { v: 10 }, Cell { v: 20 }, Cell { v: 30 }, Cell { v: 40 })
}

fn at_get(i: usize) -> u64 {
    Array.of[u64](10, 20, 30, 40).get(i) || 0
}

fn at_bytes(i: usize) -> u8 {
    "abcd".toBytes().get(i) || 0
}

fn at_with(i: usize) -> u64 {
    cells().with(i, (c: &Cell) => c.v) || 0
}

fn at_with_mut(i: usize) -> u64 {
    let mut xs = cells()
    _ = xs.withMut(i, (c: &mut Cell) => {
        c.v = 1
    })
    xs.len().toU64()
}

fn at_take(i: usize) -> u64 {
    let mut xs = Array.of[u64](10, 20, 30, 40)
    xs.takeAt(i) || 0
}

fn at_insert(i: usize) -> usize {
    let mut xs = Array.of[u64](10, 20, 30, 40)
    _ = xs.tryInsertAt(i, 5)
    xs.len()
}

fn at_slice(i: usize) -> u64 {
    let xs = Array.of[u64](10, 20, 30, 40)
    _ = xs.trySlice(i, 4)
    1
}

fn at_bytes_slice(i: usize) -> u64 {
    _ = "abcd".toBytes().trySlice(i, 4)
    1
}

fn at_string_slice(i: usize) -> u64 {
    _ = "abcd".trySliceBytes(i, 4)
    1
}

fn at_take_prefix(i: usize) -> u64 {
    _ = Array.of[u64](10, 20, 30, 40).take(i)
    1
}

fn at_buffer(i: usize) -> u64 {
    let mut b = ByteBuffer.tryNew(4).unwrap()
    b.set(i, 1)
    1
}

fn main() {
    _ = at_get(0)
    _ = at_bytes(0)
    _ = at_with(0)
    _ = at_with_mut(0)
    _ = at_take(0)
    _ = at_insert(0)
    _ = at_slice(0)
    _ = at_bytes_slice(0)
    _ = at_string_slice(0)
    _ = at_take_prefix(0)
    _ = at_buffer(0)
}
`

// TestSecretSanIndexPrimitives runs every library index primitive with a secret index: the shadow mode
// must refuse it and the two-run trace must tell index 0 from index 2.
func TestSecretSanIndexPrimitives(t *testing.T) {
	prog := secretSanProgram(t, secretSanIndexSrc)
	for _, fn := range []string{
		"at_get", "at_bytes", "at_with", "at_with_mut", "at_take", "at_insert",
		"at_slice", "at_bytes_slice", "at_string_slice", "at_take_prefix", "at_buffer",
	} {
		t.Run(fn, func(t *testing.T) {
			if _, err := vm.SecretSanRun(prog, vm.Options{}, fn, []int64{2}, []bool{true}, nil, nil); err == nil || !strings.Contains(err.Error(), "reaches an index") {
				t.Errorf("shadow mode: %v", err)
			}
			if _, err := vm.SecretSanRun(prog, vm.Options{}, fn, []int64{2}, []bool{false}, nil, nil); err != nil {
				t.Errorf("public index refused: %v", err)
			}
			err := vm.CompareTraces(prog, vm.Options{}, fn, []int64{0}, []int64{2}, nil, nil)
			if err == nil || !strings.Contains(err.Error(), "object-relative indices") {
				t.Errorf("two-run trace: %v", err)
			}
			if err := vm.CompareTraces(prog, vm.Options{}, fn, []int64{2}, []int64{2}, nil, nil); err != nil {
				t.Errorf("equal inputs traced differently: %v", err)
			}
		})
	}
}
