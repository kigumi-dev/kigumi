package plan_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	mirreach "kigumi/internal/compiler/mir/reach"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/devkit/mirfixture"
)

const simdValueSrc = `import simd from std/simd
import task from std/task
async fn g[T: Copy](v: simd.Vec[T, 4]) -> simd.Vec[T, 4] {
    await task.yieldNow()
    v.add(v)
}
type B[T] = { pub v simd.Vec[T, 4] }
fn B[T: Copy].first(self) -> T { self.v.add(self.v).lane(0) }
fn main() {
    let f = g[i32]
    let b = B[i32] { v: simd.Vec.splat[i32, 4](1) }
    let m = b.first
    let ex = task.currentThreadExecutor()
    print "${ex.runToCompletion(f(simd.Vec.splat[i32, 4](3))).lane(0)} ${m()}"
}
`

func TestSpecializeSimdFunctionValues(t *testing.T) {
	p := specialized(t, simdValueSrc)
	main := mirfixture.FuncNamed(t, p, "main.main")
	values := 0
	for _, blk := range main.Blocks {
		for _, in := range blk.Insts {
			if in.Op != midir.OpFnItem && in.Form() != midir.FormMethod {
				continue
			}
			inst, ok := p.R.InstanceOf(in.Ent)
			if !ok {
				t.Fatalf("%s names the shared body of %s", in.Op, p.R.Entity(in.Ent).Name)
			}
			if !p.InstanceOnly[inst.Gen] {
				t.Errorf("%s is not recorded as running only as instances", p.R.Entity(inst.Gen).Name)
			}
			values++
		}
	}
	if values != 2 {
		t.Fatalf("specialized %d function values, want 2", values)
	}
	if claims := mirverify.CheckSimdShapes(p, mirreach.Reachable(p.R, p.Entry, p.Funcs, p.ByEnt)); len(claims) != 0 {
		t.Fatalf("shared SIMD bodies stay reachable: %+v", claims)
	}
	if err := mirverify.Program(p); err != nil {
		t.Fatal(err)
	}
}
