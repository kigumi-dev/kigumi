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

// The VM sweeps a callback whose captures cannot own anything without counting it, so only a release through a freed object shows here.
func TestUnwoundStdCallReleasesNothingTwice(t *testing.T) {
	quiesce()
	for _, s := range []struct{ name, setup, call string }{
		{"direct", stdCallLeakArray, `xs.tryMap((x: &Int) => if x == 2 { explode() } else { x + k })`},
		{"method value", stdCallLeakTable, `callW(table.with, &h, k)`},
	} {
		t.Run(s.name, func(t *testing.T) {
			src := fmt.Sprintf("%s%s    let r = %s\n    0\n%s", stdCallLeakHead, s.setup, s.call, stdCallLeakTail)
			if _, _, freedUses, _ := runTracked(t, src); freedUses != 0 {
				t.Errorf("%d releases through a freed object", freedUses)
			}
		})
	}
}
