package mir_test

import (
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

const specMethodValueSrc = `
type Res = resource {
    pub n Int
}

fn Res.drop(move self) -> Unit {}

fn Res.get(self, k: Int) -> Int { self.n + k }

interface Getter {
    fn get(self, k: Int) -> Int
}

type Point = {
    pub x Int
    pub y Int
}

fn viaParam[T: Getter, P: Copy](t: T, p: P) -> P {
    let f = t.get
    let n = f(1)
    let g = (k: Int) => f(k)
    let m = n + g(2)
    p
}

fn run(x: Int) -> Int {
    viaParam(Res { n: x }, Point { x: x, y: x }).y
}

fn main() -> Unit! {}
`

func TestSpecializeMintsAMethodValueOverATypeParameter(t *testing.T) {
	p := specialized(t, specMethodValueSrc)
	body := funcNamed(t, p, "main.viaParam[main.Res,main.Point]")
	built, ok := firstOp(body, mir.OpClosure)
	if !ok || built.Str != "witness" {
		t.Fatalf("no method value in the instantiation:\n%s", p.Dump())
	}
	ty := body.Locals[built.Dst].Type
	if p.R.Types.Kind(ty) != sem.KClosure {
		t.Fatalf("method value typed %s, want a closure", p.R.TypeString(ty))
	}
	info := p.R.Closure(p.R.Types.Node(ty).Ent)
	if !info.IsMethodValue() || len(info.Captures) != 1 {
		t.Fatalf("closure is not a method value with one capture: %+v", info)
	}
	if got := p.R.TypeString(p.R.Entity(info.Captures[0].Local).Type); got != "Res" {
		t.Errorf("captured receiver typed %s, want Res", got)
	}
	if err := p.Verify(); err != nil {
		t.Errorf("verify: %v", err)
	}
}
