package driver_test

import (
	"fmt"
	"testing"
)

// %[1]s is the cell's type, %[2]s its first content, %[3]s one round's atomic, %[4]d the round count.
const atomicRoundSrc = `import mem from std/mem
import ffi from std/ffi

fn run(c: &mem.UnsafeCell[%[1]s], n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        %[3]s
    }
    s
}

let c = mem.UnsafeCell.new(%[2]s)
print "ok=${run(&c, %[4]d)}"
`

var atomicRoundCases = []struct {
	name, ty, init, round string
	perRound              int
}{
	{"int load", "Int", "10000", "if unsafe { c.atomicLoad(mem.AtomicOrder.Relaxed) } > 0 { s = s + 1 }", 1},
	{"int store", "Int", "10000", "unsafe { c.atomicStore(20000, mem.AtomicOrder.Relaxed) }", 2},
	{"int exchange", "Int", "10000", "if unsafe { c.atomicExchange(20000, mem.AtomicOrder.Relaxed) } > 0 { s = s + 1 }", 2},
	{"int compare exchange", "Int", "10000", "_ = unsafe { c.atomicCompareExchange(10000, 10000, mem.AtomicOrder.Relaxed) }", 2},
	{"int add", "Int", "10000", "if unsafe { c.atomicAdd(1, mem.AtomicOrder.Relaxed) } > 0 { s = s + 1 }", 2},
	{"pointer load", "*const u8", "ffi.null[u8]()", "if ffi.isNull(unsafe { c.atomicLoad(mem.AtomicOrder.Relaxed) }) { s = s + 1 }", 1},
}

// The extra rounds differ only in their atomics, so the cell's setup cancels out.
func TestFaultInjectionAtomicBoxSitesCountTheSameOnBothBackends(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range atomicRoundCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			few := fmt.Sprintf(atomicRoundSrc, c.ty, c.init, c.round, boxSiteRounds)
			many := fmt.Sprintf(atomicRoundSrc, c.ty, c.init, c.round, 2*boxSiteRounds)
			vmBoxes := vmAllocs(t, many) - vmAllocs(t, few)
			nativeBoxes := nativeAllocs(t, many) - nativeAllocs(t, few)
			if want := boxSiteRounds * c.perRound; vmBoxes != want || nativeBoxes != want {
				t.Errorf("%d more rounds cost %d allocations on the VM and %d on native, want %d", boxSiteRounds, vmBoxes, nativeBoxes, want)
			}
		})
	}
}
