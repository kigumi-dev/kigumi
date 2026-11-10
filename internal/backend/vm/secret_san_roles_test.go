package vm_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

// sinkProbe passes the secret s to one integer argument of one primitive or builtin.
type sinkProbe struct {
	op   string
	at   int
	role string
	body string
}

var sinkProbes = []sinkProbe{
	{"array.Array.get", 1, "index", "_ = Array.of[u64](1, 2).get(s)"},
	{"array.Array.with", 1, "index", "_ = cells().with(s, (c: &Cell) => c.v)"},
	{"array.Array.withMut", 1, "index", "let mut xs = cells()\n    _ = xs.withMut(s, (c: &mut Cell) => { c.v = 1 })"},
	{"array.Array.takeAt", 1, "index", "let mut xs = Array.of[u64](1, 2)\n    _ = xs.takeAt(s)"},
	{"array.Array.tryInsertAt", 1, "index", "let mut xs = Array.of[u64](1, 2)\n    _ = xs.tryInsertAt(s, 3)"},
	{"array.Array.trySlice", 1, "index", "_ = Array.of[u64](1, 2).trySlice(s, 2)"},
	{"array.Array.trySlice", 2, "index", "_ = Array.of[u64](1, 2).trySlice(0, s)"},
	{"array.Array.tryCopyRange", 2, "index", "_ = Array.of[u64](1, 2).take(s)"},
	{"prelude.Bytes.get", 1, "index", "_ = \"ab\".toBytes().get(s)"},
	{"prelude.Bytes.trySlice", 1, "index", "_ = \"ab\".toBytes().trySlice(s, 2)"},
	{"prelude.Bytes.trySlice", 2, "index", "_ = \"ab\".toBytes().trySlice(0, s)"},
	{"prelude.String.trySliceBytes", 1, "index", "_ = \"ab\".trySliceBytes(s, 2)"},
	{"prelude.String.trySliceBytes", 2, "index", "_ = \"ab\".trySliceBytes(0, s)"},
	{"prelude.ByteBuffer.set", 1, "index", "let mut b = ByteBuffer.tryNew(4).unwrap()\n    b.set(s, 1)"},
	{"prelude.ByteBuffer.copyFrom", 1, "index", "let mut b = ByteBuffer.tryNew(4).unwrap()\n    b.copyFrom(s, \"ab\".toBytes(), 0, 1)"},
	{"prelude.ByteBuffer.copyFrom", 3, "index", "let mut b = ByteBuffer.tryNew(4).unwrap()\n    b.copyFrom(0, \"ab\".toBytes(), s, 2)"},
	{"prelude.ByteBuffer.copyFrom", 4, "index", "let mut b = ByteBuffer.tryNew(4).unwrap()\n    b.copyFrom(0, \"ab\".toBytes(), 0, s)"},
	{"prelude.ByteBuffer.copyFromString", 1, "index", "let mut b = ByteBuffer.tryNew(4).unwrap()\n    b.copyFromString(s, \"ab\", 0, 1)"},
	{"prelude.ByteBuffer.copyFromString", 3, "index", "let mut b = ByteBuffer.tryNew(4).unwrap()\n    b.copyFromString(0, \"ab\", s, 2)"},
	{"prelude.ByteBuffer.copyFromString", 4, "index", "let mut b = ByteBuffer.tryNew(4).unwrap()\n    b.copyFromString(0, \"ab\", 0, s)"},
	{"array.Array.growBy", 1, "size", "let mut xs = Array.of[u64](1, 2)\n    _ = xs.tryReserve(s)"},
	{"array.Array.tryWithCapacity", 0, "size", "_ = Array.tryWithCapacity[u64](s)"},
	{"prelude.Bytes.tryZeros", 0, "size", "_ = Bytes.tryZeros(s)"},
	{"prelude.ByteBuffer.tryNew", 0, "size", "_ = ByteBuffer.tryNew(s)"},
	{"entropy.Entropy.bytes", 1, "size", "_ = host.entropy().bytes(s)"},
	{"task.threadPoolExecutor", 0, "size", "_ = task.threadPoolExecutor(s)"},
	{"array.Array.tryFilter", 1, "branch", "_ = Array.of[usize](1, 2).filter((v: &usize) => v == s)"},
	{"array.trySortedBy", 1, "branch", "_ = sort.stable(&Array.of[usize](1, 2), (a: &usize, b: &usize) => s.compareTo(a))"},
	{"alloc.Block.tryNew", 0, "size", "_ = alloc.Block.tryNew(s)"},
	{"alloc.Block.get", 1, "index", "let b = alloc.Block.new(4)\n    _ = b.get(s)"},
	{"alloc.Block.set", 1, "index", "let mut b = alloc.Block.new(4)\n    b.set(s, 1)"},
	{"volatile.read", 0, "address", "_ = unsafe { mem.volatileReadAt[u64](s) }"},
	{"volatile.write", 0, "address", "unsafe { mem.volatileWriteAt(s, 1) }"},
}

// unreachableSinks are roles no Kigumi program can feed a secret: the table keeps them for a caller that could.
var unreachableSinks = map[string]string{
	"array.Array.tryCopyRange#1":   "std passes a constant start",
	"prelude.FixedArray.withMut#1": "a direct call is lowered to a bounds branch, which is the sink",
	"fixed.fill#1":                 "the length is a const generic",
	"iter.at#1":                    "the loop counter",
	"iter.at_ref#1":                "the loop counter",
	"iter.at_move#1":               "the loop counter",
	"mem.volatileReadAt#0":         "only callable directly, which is the volatile.read builtin",
	"mem.volatileWriteAt#0":        "only callable directly, which is the volatile.write builtin",
}

var sinkText = map[string]string{"index": "an index", "size": "an allocation size", "address": "an address", "branch": "a conditional branch"}

// TestSinkRolesHaveProbes feeds a secret to every index, size, address and branching-callback argument the tables
// name: dropping a role from a table fails its probe, and a role without a probe fails here.
func TestSinkRolesHaveProbes(t *testing.T) {
	var src strings.Builder
	src.WriteString("import alloc from std/alloc\nimport mem from std/mem\nimport sort from std/sort\nimport task from std/task\n\ntype Cell = {\n    pub mut v u64\n}\n\n")
	src.WriteString("fn cells() -> Array[Cell] {\n    Array.of(Cell { v: 1 }, Cell { v: 2 })\n}\n\n")
	for i, p := range sinkProbes {
		fmt.Fprintf(&src, "fn probe%d(s: usize) {\n    %s\n}\n\n", i, p.body)
	}
	src.WriteString("fn main() {\n")
	for i := range sinkProbes {
		fmt.Fprintf(&src, "    _ = probe%d\n", i)
	}
	src.WriteString("}\n")
	prog := secretSanProgram(t, src.String())
	roles := vm.SinkRoles()
	var probed []string
	for i, p := range sinkProbes {
		key := fmt.Sprintf("%s#%d", p.op, p.at)
		probed = append(probed, key)
		if roles[key] != p.role {
			t.Errorf("%s: the tables give role %q, the probe expects %q", key, roles[key], p.role)
		}
		_, err := vm.SecretSanRun(prog, vm.Options{}, fmt.Sprintf("probe%d", i), []int64{2}, []bool{true}, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "SecretSan: secret value reaches "+sinkText[p.role]) {
			t.Errorf("%s: a secret argument gave %v, want the %s sink", key, err, p.role)
		}
	}
	for key := range roles {
		if !slices.Contains(probed, key) && unreachableSinks[key] == "" {
			t.Errorf("%s has a sink role but no probe", key)
		}
	}
}
