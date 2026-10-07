package vm_test

import (
	"bytes"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/vm"
)

const crossLoopSrc = `type P = { pub x Int, pub y Int, pub z Int, pub w Int }

fn pick[T](v: T) -> T { v }

fn mk(i: Int) -> P { P { x: i, y: i, z: i, w: i } }

fn main() -> Unit! {
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
	prog := mir.Build(res)
	err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}, mir.Transform{Name: "plan-direct", Run: mir.PlanDirect})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	opts := vm.Options{Sandbox: true, StepLimit: 1 << 40, DepthLimit: 100, MemoryLimit: 1 << 20}
	code, err := vm.Run(prog, opts, &out, &errOut)
	if err != nil || code != 0 || out.String() != "9999900000\n" {
		t.Fatalf("exit %d err %v stdout %q stderr %q", code, err, out.String(), errOut.String())
	}
}
