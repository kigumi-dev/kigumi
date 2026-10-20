package vm_test

import (
	"fmt"
	"testing"
)

const stdCallLeakHead = `import task from std/task
import ffi from std/ffi

fn explode() -> Int {
    None.expect("boom")
}

fn callW[F: Fn[fn(&ffi.FfiHandle[Int], fn(&Int) -> Int) -> Int]](w: F, h: &ffi.FfiHandle[Int], k: Int) -> Int {
    w(h, (v: &Int) => if v == 41 { explode() } else { v + k })
}

async fn boom() -> Int {
`

const stdCallLeakTail = `}

async fn root() -> Int {
    let h = task.spawn(boom())
    await h
}

let ex = task.currentThreadExecutor()
print "total ${ex.runToCompletion(root())}"
`

const (
	stdCallLeakArray = `    let xs = Array.of(1, 2, 3)
    let k = 7
`
	stdCallLeakTable = `    let table: ffi.HandleTable[Int] = ffi.HandleTable.new()
    let h = table.insert(41)
    let k = 7
`
)

func stdCallLeakSrc(setup, call string) string {
	return fmt.Sprintf("%s%s    let r = %s\n    0\n%s", stdCallLeakHead, setup, call, stdCallLeakTail)
}

// The panic itself leaves a fixed set of objects, so a call that adds any is leaking.
func TestUnwoundStdCallLeavesNothingBehind(t *testing.T) {
	quiesce()
	for _, s := range []struct{ name, setup, call string }{
		{"direct", stdCallLeakArray, `xs.tryMap((x: &Int) => if x == 2 { explode() } else { x + k })`},
		{"method value", stdCallLeakTable, `callW(table.with, &h, k)`},
	} {
		t.Run(s.name, func(t *testing.T) {
			_, base, _, _ := runTracked(t, stdCallLeakSrc(s.setup, `explode()`))
			_, live, freedUses, _ := runTracked(t, stdCallLeakSrc(s.setup, s.call))
			if freedUses != 0 {
				t.Errorf("%d releases through a freed object", freedUses)
			}
			if live != base {
				t.Errorf("%d objects still live, %d when the call is replaced by the panic itself", live, base)
			}
		})
	}
}
