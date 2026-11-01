package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/toolchain/driver"
)

// Every atomic is followed by a marker line, so a refused allocation shows which atomic took it.
const atomicOpsSrc = `import mem from std/mem
import ffi from std/ffi

fn main() {
    let n = mem.UnsafeCell.new(10000)
    let p = mem.UnsafeCell.new(ffi.null[u8]())
    let b = mem.UnsafeCell.new(true)
    eprint "armed"
    unsafe {
%s    }
}
`

// ucell_atomic boxes the old content it returns, then the value it stores: rt_int outside its cache, rt_ptr always, a Bool never.
var atomicOpBoxes = []struct {
	op    string
	boxes int
}{
	{"_ = n.atomicLoad(mem.AtomicOrder.Relaxed)", 1},
	{"n.atomicStore(20000, mem.AtomicOrder.Relaxed)", 2},
	{"_ = n.atomicExchange(30000, mem.AtomicOrder.Relaxed)", 2},
	{"_ = n.atomicCompareExchange(30000, 40000, mem.AtomicOrder.Relaxed)", 2},
	{"_ = n.atomicCompareExchange(1, 2, mem.AtomicOrder.Relaxed)", 1},
	{"_ = n.atomicAdd(1, mem.AtomicOrder.Relaxed)", 2},
	{"_ = n.atomicSub(40001, mem.AtomicOrder.Relaxed)", 1},
	{"_ = n.atomicOr(5, mem.AtomicOrder.Relaxed)", 0},
	{"_ = n.atomicXor(20000, mem.AtomicOrder.Relaxed)", 1},
	{"_ = n.atomicAnd(255, mem.AtomicOrder.Relaxed)", 1},
	{"n.atomicStore(7, mem.AtomicOrder.Relaxed)", 0},
	{"_ = p.atomicLoad(mem.AtomicOrder.Relaxed)", 1},
	{"_ = b.atomicLoad(mem.AtomicOrder.Relaxed)", 0},
	{"_ = b.atomicExchange(false, mem.AtomicOrder.Relaxed)", 0},
	{"b.atomicStore(true, mem.AtomicOrder.Relaxed)", 0},
	{"_ = b.atomicCompareExchange(true, false, mem.AtomicOrder.Relaxed)", 0},
}

func atomicOpsModule(t *testing.T) (*driver.Module, []int) {
	t.Helper()
	var body strings.Builder
	want := make([]int, len(atomicOpBoxes))
	for i, c := range atomicOpBoxes {
		fmt.Fprintf(&body, "        %s\n        print \"%d\"\n", c.op, i)
		want[i] = c.boxes
	}
	return loadSweepModule(t, fmt.Sprintf(atomicOpsSrc, body.String()), ""), want
}

// atomicOpAllocs refuses allocation 1, 2, ... after arming until a run finishes, and counts the refusals each atomic took.
func atomicOpAllocs(t *testing.T, want []int, run func(n int) (code int, out, errOut string)) []int {
	t.Helper()
	got := make([]int, len(want))
	limit := 1
	for _, w := range want {
		limit += w
	}
	for n := 1; n <= limit; n++ {
		code, out, errOut := run(n)
		done := strings.Count(out, "\n")
		switch {
		case code == 0 && done == len(got):
			return got
		case code == 2 && strings.Contains(errOut, "out of memory") && done < len(got):
			got[done]++
		default:
			t.Fatalf("refusing allocation %d: exit %d, stdout=%q stderr=%q", n, code, out, errOut)
		}
	}
	t.Fatalf("still refusing at allocation %d, after %v", limit, got)
	return nil
}

func TestAtomicCellBoxSitesAllocateNatively(t *testing.T) {
	t.Parallel()
	m, want := atomicOpsModule(t)
	exe := buildFaultInjected(t, m)
	got := atomicOpAllocs(t, want, func(n int) (int, string, string) {
		cmd := exec.Command(exe)
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_ARM=1", "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		return exitCodeOf(cmd.Run()), out.String(), errOut.String()
	})
	if !slices.Equal(got, want) {
		t.Errorf("native allocations per atomic: got %v, want %v", got, want)
	}
}

func TestAtomicCellBoxSitesCountTheSameOnTheVM(t *testing.T) {
	t.Parallel()
	m, want := atomicOpsModule(t)
	a, err := driver.Analyze(m)
	if err != nil || a.Res.HasErrors() {
		t.Fatalf("analyze: %v", err)
	}
	got := atomicOpAllocs(t, want, func(n int) (int, string, string) {
		var out, errOut bytes.Buffer
		code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: int64(n), FaultArmOnEprint: true}, &out, &errOut)
		if err != nil {
			t.Fatalf("vm N=%d: %v", n, err)
		}
		return code, out.String(), errOut.String()
	})
	if !slices.Equal(got, want) {
		t.Errorf("VM allocations per atomic: got %v, want %v", got, want)
	}
}
