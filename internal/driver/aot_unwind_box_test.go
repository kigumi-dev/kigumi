package driver_test

import (
	"fmt"
	"testing"
)

const boxLeakHead = `import task from std/task
import ffi from std/ffi

fn explode() -> Int {
    None.expect("boom")
}
`

const boxLeakTail = `
async fn root() -> Int {
    let h = task.spawn(boom())
    await h
}

let ex = task.currentThreadExecutor()
print "total ${ex.runToCompletion(root())}"
`

const (
	boxLeakArray = `    let xs = Array.of(1, 2, 3)
    let k = 7
`
	boxLeakTable = `    let table: ffi.HandleTable[Int] = ffi.HandleTable.new()
    let h = table.insert(41)
    let k = 7
    let w: fn(&ffi.FfiHandle[Int], fn(&Int) -> Int) -> Int = table.with
`
)

type boxLeakShape struct {
	name, setup, call, bare string
}

var boxLeakShapes = []boxLeakShape{
	{"direct", boxLeakArray, `xs.tryMap((x: &Int) => if x == 2 { explode() } else { x + k })`, `explode()`},
	{"method value", boxLeakTable, `w(&h, (v: &Int) => if v == 41 { explode() } else { v + k })`, `explode()`},
}

func boxLeakSrc(s boxLeakShape, call string, async bool) string {
	if !async {
		return fmt.Sprintf("%sfn scan() -> Int {\n%s    let r = %s\n    0\n}\n\nasync fn boom() -> Int {\n    scan()\n}\n%s", boxLeakHead, s.setup, call, boxLeakTail)
	}
	return fmt.Sprintf("%sasync fn boom() -> Int {\n%s    let r = %s\n    0\n}\n%s", boxLeakHead, s.setup, call, boxLeakTail)
}

func TestUnwoundStdCallLeavesNoBoxBehindNative(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, s := range boxLeakShapes {
		for _, async := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/async=%v", s.name, async), func(t *testing.T) {
				t.Parallel()
				got := liveAfterFault(t, boxLeakSrc(s, s.call, async))
				want := liveAfterFault(t, boxLeakSrc(s, s.bare, async))
				if got != want {
					t.Errorf("%s objects still held at exit, %s when the call is replaced by the panic itself", got, want)
				}
			})
		}
	}
}
