package vm_test

import (
	"bytes"
	"math"
	"testing"

	"kigumi/internal/backend/vm"
	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
)

const crossLoopSrc = `type P = { pub x Int, pub y Int, pub z Int, pub w Int }

fn pick[T](v: T) -> T { v }

fn mk(i: Int) -> P { P { x: i, y: i, z: i, w: i } }

fn main() -> Void! {
    let mut s = 0
    let mut i = 0
    let mut arr: Array[P] = Array.empty()
    arr.push(mk(0))
    for i < 100000 {
        let p = pick(mk(i))
        arr[0] = p
        let q = arr[0]
        s = s + q.x + p.y
        i = i + 1
    }
    print "${s}"
}
`

func TestDirectCrossingsKeepTheLiveTotalFlat(t *testing.T) {
	res := checkSource(t, crossLoopSrc)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}, mirplan.Transform{Name: "plan-direct", Run: mirplan.PlanDirect})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	opts := vm.Options{Sandbox: true, StepLimit: math.MaxInt, DepthLimit: 100, MemoryLimit: 1 << 20}
	code, err := vm.Run(prog, opts, &out, &errOut)
	if err != nil || code != 0 || out.String() != "9999900000\n" {
		t.Fatalf("exit %d err %v stdout %q stderr %q", code, err, out.String(), errOut.String())
	}
}
