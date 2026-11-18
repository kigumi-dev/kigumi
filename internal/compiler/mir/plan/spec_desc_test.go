package plan_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/compiler/sem"
)

// memberNamed is the method name of type owner, found by its receiver.
func memberNamed(r *sem.Result, owner sem.EntityID, name string) sem.EntityID {
	for id := range r.Entities {
		ent := sem.EntityID(id)
		if _, minted := r.InstanceOf(ent); minted || r.Entity(ent).Name != name || r.Entity(ent).Kind != sem.EntFn || r.Fn(ent).SelfParam == 0 {
			continue
		}
		self := r.Entity(r.Fn(ent).SelfParam).Type
		if r.Types.Kind(self) == sem.KRef {
			self = r.Types.Node(self).Elem
		}
		if r.Types.Node(self).Ent == owner {
			return ent
		}
	}
	return 0
}

func TestSpecializeDescriptorBodiesPerInstance(t *testing.T) {
	p := specialized(t, `import simd from std/simd
interface Eval { fn value(self) -> Int }
type Wrap[const N: usize] = { pub v simd.Vec[i32, N] }
fn Wrap[N].value(self) -> Int { self.v.add(self.v).lane(0).toInt() }
type Guard[const N: usize] = resource { pub v simd.Vec[i32, N] }
fn Guard[N].drop(move self) { _ = self.v.add(self.v) }
fn show(x: Eval) -> Int { x.value() }
fn main() {
    print "${show(Wrap { v: simd.Vec.splat[i32, 4](1) })}"
    let g = Guard { v: simd.Vec.splat[i32, 4](2) }
}
`)
	if len(p.Diags) != 0 {
		t.Fatalf("diagnostics: %+v", p.Diags)
	}
	var wrap, guard sem.TypeID
	for _, f := range p.Funcs {
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				switch {
				case in.Op == midir.OpBox && p.R.Entity(p.R.Types.Node(in.Type).Ent).Name == "Wrap":
					wrap = in.Type
				case in.Op == midir.OpRecord && p.R.Entity(in.Ent).Name == "Guard":
					guard = f.Locals[in.Dst].Type
				}
			}
		}
	}
	if wrap == 0 || guard == 0 {
		t.Fatalf("no Wrap box or Guard record:\n%s", p.Dump())
	}
	value := memberNamed(p.R, p.R.Types.Node(wrap).Ent, "value")
	drop := p.R.TypeDecl(p.R.Types.Node(guard).Ent).Drop
	for _, c := range []struct {
		fn   sem.EntityID
		inst sem.TypeID
	}{{value, wrap}, {drop, guard}} {
		body := p.Desc.Get(p.R, c.fn, c.inst)
		if body == nil {
			t.Fatalf("no descriptor body of %s for %s", p.R.Entity(c.fn).Name, p.R.TypeString(c.inst))
		}
		if in, ok := p.R.InstanceOf(body.Ent); !ok || in.Gen != c.fn {
			t.Errorf("%s runs %s, not an instantiation of it", p.R.Entity(c.fn).Name, body.Name)
		}
	}
	if err := mirverify.Program(p); err != nil {
		t.Fatal(err)
	}
}

func TestBoxOfAParameterComposesItsRuntimeType(t *testing.T) {
	p := specialized(t, `type Fault[T] = { pub value T }
fn Fault[T].message(self) -> String { "fault" }
fn mk[T](v: T) -> Error { Fault { value: v } }
fn main() {
    let a = mk(1)
    let b = mk("s")
}
`)
	if len(p.Diags) != 0 {
		t.Fatalf("diagnostics: %+v", p.Diags)
	}
	mk := funcNamed(t, p, "mk")
	if !mk.HasEnv {
		t.Fatalf("mk boxes Fault[T] without its environment:\n%s", p.Dump())
	}
	for _, blk := range mk.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpBox && len(in.Rt) != 1 {
				t.Errorf("mk's box of %s names no runtime type", p.R.TypeString(in.Type))
			}
		}
	}
}
