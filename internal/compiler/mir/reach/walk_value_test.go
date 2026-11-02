package reach_test

import (
	"testing"

	mirbuild "kigumi/internal/compiler/mir/build"
	mirreach "kigumi/internal/compiler/mir/reach"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

const constReachSrc = `
type Leaf = { x Int, y Int }
type Mid = { leaf Leaf, m Int }
type Top = { mid Mid, n Int }
type Inner = { v Int }
type Outer = { inner Inner, w Int }

fn mkTop() -> Top { Top { mid: Mid { leaf: Leaf { x: 1, y: 2 }, m: 3 }, n: 4 } }
fn mkOuter() -> Outer { Outer { inner: Inner { v: 1 }, w: 2 } }

const top: Top = comptime mkTop()
const outer: Outer = comptime mkOuter()

fn unused() -> Int { outer.w }

fn main() {
    print "${top.n}"
}
`

func constThunk(t *testing.T, r *sem.Result, name string) sem.EntityID {
	t.Helper()
	for id := 1; id < len(r.Entities); id++ {
		if e := r.Entity(sem.EntityID(id)); e.Kind == sem.EntConst && e.Name == name {
			return r.ConstThunk(sem.EntityID(id))
		}
	}
	t.Fatalf("no const named %q", name)
	return 0
}

func TestReachableKeepsConstValueTypes(t *testing.T) {
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(constReachSrc)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	values := map[sem.EntityID]sem.Literal{
		constThunk(t, res, "top"):   recordLit(recordLit(recordLit(intLit(1), intLit(2)), intLit(3)), intLit(4)),
		constThunk(t, res, "outer"): recordLit(recordLit(intLit(1)), intLit(2)),
	}
	for _, cf := range prog.Comptime {
		if lit, ok := values[cf.Owner.Ent]; ok {
			prog.Fold(cf, lit)
		}
	}
	rs := mirreach.Reachable(prog.R, prog.Entry, prog.Funcs, prog.ByEnt)
	for _, name := range []string{"Top", "Mid", "Leaf"} {
		if !rs.KeepType(entType(t, res, name)) {
			t.Errorf("%s: held by a constant reachable code reads, should get a descriptor although no reachable code builds it", name)
		}
	}
	for _, name := range []string{"Outer", "Inner"} {
		if rs.KeepType(entType(t, res, name)) {
			t.Errorf("%s: held only by a constant unreachable code reads, should not get a descriptor", name)
		}
	}
}
