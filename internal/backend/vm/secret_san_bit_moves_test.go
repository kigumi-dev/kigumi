package vm_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

func TestSecretSanBitMoves(t *testing.T) {
	prog := secretSanProgram(t, `
fn add(s: u64) -> Bool { ((s & 1).wrappingAdd(1) & 2) != 0 }
fn rotate(s: u64) -> Bool { ((s & 1).rotateLeft(3) & 8) != 0 }
fn extend(s: i8) -> Bool { (s.toInt() & 256) != 0 }
fn shift(s: i8) -> Bool { ((s >> 7) & 2) != 0 }
fn wideShift(s: i128) -> Bool { ((s >> 80) & (1 << 70)) != 0 }
fn narrow(s: u64) -> Bool { s.toU8() is Some(_) }
fn safeNarrow(s: u64) -> u64 {
    let b = (s & 255).toU8()
    if b is Some(_) { (b || 0).toU64() } else { 0 }
}
fn addBranch(s: u64) -> Int { if add(s) { 1 } else { 0 } }
fn rotateBranch(s: u64) -> Int { if rotate(s) { 1 } else { 0 } }
fn extendBranch(s: i8) -> Int { if extend(s) { 1 } else { 0 } }
fn shiftBranch(s: i8) -> Int { if shift(s) { 1 } else { 0 } }
fn wideShiftBranch(s: i128) -> Int { if wideShift(s) { 1 } else { 0 } }
fn narrowBranch(s: u64) -> Int { if narrow(s) { 1 } else { 0 } }
fn main() {
    _ = addBranch(1)
    _ = rotateBranch(1)
    _ = extendBranch(-1)
    _ = shiftBranch(-1)
    _ = wideShiftBranch(-1)
    _ = narrowBranch(300)
    _ = safeNarrow(300)
}
`)
	for _, tc := range []struct {
		name string
		arg  int64
	}{
		{"addBranch", 1}, {"rotateBranch", 1}, {"extendBranch", -1}, {"shiftBranch", -1}, {"wideShiftBranch", -1}, {"narrowBranch", 300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := vm.SecretSanRun(prog, vm.Options{}, tc.name, []int64{tc.arg}, []bool{true}, nil, nil)
			if err == nil || !strings.Contains(err.Error(), "SecretSan: secret value reaches a conditional branch") {
				t.Fatalf("branch: %v", err)
			}
		})
	}
	sh, err := vm.SecretSanRun(prog, vm.Options{}, "safeNarrow", []int64{300}, []bool{true}, nil, nil)
	if err != nil || !sh.Tainted() {
		t.Fatalf("masked conversion: shadow=%+v err=%v", sh, err)
	}
}
