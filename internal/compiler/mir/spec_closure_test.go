package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

const specClosureSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn hold[T: Copy](x: T) -> T {
    let g = () => x
    g()
}

fn twice[T: Copy](x: T) -> (T, T) {
    let g = () => x
    let h = () => g()
    (g(), h())
}

fn later[T: Copy](x: T) -> fn() -> T {
    () => x
}

fn run(x: Int) -> Int {
    let p = Point { x: x, y: x }
    hold(p).x + twice(p)._0.y + later(p)().x
}

fn main() -> Void! {}
`

func closureFuncs(p *mir.Program, prefix string) []*midir.Func {
	var out []*midir.Func
	for _, f := range p.Funcs {
		if strings.HasPrefix(f.Name, prefix) && p.R.Entity(f.Ent).Kind == sem.EntClosure {
			out = append(out, f)
		}
	}
	return out
}

func TestSpecializeCopiesAClosureBuiltInTheBody(t *testing.T) {
	p := specialized(t, specClosureSrc)
	body := funcNamed(t, p, "main.hold[main.Point]")
	clo := funcNamed(t, p, "main.hold.closure1[main.Point]")
	built, ok := firstOp(body, midir.OpClosure)
	if !ok || built.Ent != clo.Ent {
		t.Fatalf("the instantiation builds %d, want the copy %d:\n%s", built.Ent, clo.Ent, p.Dump())
	}
	if p.R.ClosureOrigin(clo.Ent) == clo.Ent || p.ByEnt[clo.Ent] != clo {
		t.Errorf("the copy is not a minted closure of its own")
	}
	point := clo.Locals[clo.Params[0]].Type
	if !p.R.DirectAgg(point) || clo.DSig.Param(0) != point || clo.DSig.Ret != point || !p.EnvABI(clo) {
		t.Errorf("the closure does not take the record in its environment: %+v", clo.DSig)
	}
	if !body.IsDirect(built.Dst) {
		t.Errorf("the closure is boxed in the instantiation:\n%s", p.Dump())
	}
	for _, f := range []*midir.Func{body, clo} {
		if n := countOp(f, midir.OpBoxDirect) + countOp(f, midir.OpUnboxDirect); n != 0 {
			t.Errorf("%s crosses %d times:\n%s", f.Name, n, p.Dump())
		}
	}
	if err := p.Verify(); err != nil {
		t.Errorf("verify: %v", err)
	}
}

func TestSpecializeKeepsTheSharedClosureBody(t *testing.T) {
	p := specialized(t, specClosureSrc)
	if shared := funcNamed(t, p, "main.hold.closure1"); countOp(shared, midir.OpBoxDirect) != 0 {
		t.Errorf("the shared closure body changed:\n%s", p.Dump())
	}
	generic := funcNamed(t, p, "main.hold")
	built, _ := firstOp(generic, midir.OpClosure)
	if p.R.ClosureOrigin(built.Ent) != built.Ent {
		t.Errorf("the shared body builds a copy")
	}
}

func TestSpecializeCopiesAClosureThatCapturesAClosure(t *testing.T) {
	p := specialized(t, specClosureSrc)
	first := funcNamed(t, p, "main.twice.closure1[main.Point]")
	second := funcNamed(t, p, "main.twice.closure2[main.Point]")
	held := second.Locals[second.Params[0]].Type
	if p.R.Types.Kind(held) != sem.KClosure || p.R.Types.Node(held).Ent != first.Ent {
		t.Errorf("the second closure captures %s, want the copy of the first", p.R.TypeString(held))
	}
	if err := p.Verify(); err != nil {
		t.Errorf("verify: %v", err)
	}
}

// Leaving as a plain fn value boxes the environment, so the instantiation would not pay.
func TestSpecializeLeavesAnEscapingClosureShared(t *testing.T) {
	p := specialized(t, specClosureSrc)
	for _, name := range instances(p) {
		if strings.Contains(name, "later") {
			t.Errorf("%s was instantiated", name)
		}
	}
	if got := closureFuncs(p, "main.later.closure"); len(got) != 1 {
		t.Errorf("later keeps %d closure bodies, want the shared one", len(got))
	}
}
