package mir_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

const moveClosureSrc = `
import {Box} from std/alloc

fn mkb(n: Int) -> Box[Int]? { Some(Box.new(n)) }
fn useb(a: Box[Int]?) -> Int { if a is Some(b) { b.get() } else { 0 } }

fn run() -> Int {
    let f = (a: Box[Int]?, keep: Bool) => {
        let mut r = 0
        if keep { r = useb(a) }
        r
    }
    f(mkb(3), true) + f(mkb(4), false)
}

fn main() -> Void! {}
`

const moveMapKeySrc = `
import map from std/map

type Id = { n Int }

fn run() -> Result[Void, AllocError] {
    let mut m = map.tryEmpty[Id, Int]()?
    m.trySet(Id { n: 1 }, 2)?
    Ok(())
}

fn main() {
    _ = run()
}
`

// movesBeforeCrossing wants each move out of the named local in the local's own form, with only its result crossing.
func movesBeforeCrossing(t *testing.T, p *mir.Program, f *mir.Func, name string) {
	t.Helper()
	views, moved := map[mir.LocalID]bool{}, map[mir.LocalID]bool{}
	crossed := false
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			cross := in.Op == mir.OpUnboxDirect || in.Op == mir.OpBoxDirect
			switch {
			case cross && f.Locals[in.Args[0]].Name == name:
				views[in.Dst] = true
			case cross && moved[in.Args[0]]:
				crossed = true
			case in.Op == mir.OpMove && views[in.Args[0]]:
				t.Errorf("%s moves a crossing of %%%s, which leaves the local holding its count:\n%s", f.Name, name, p.Dump())
			case in.Op == mir.OpMove && f.Locals[in.Args[0]].Name == name:
				if f.IsDirect(in.Args[0]) != f.IsDirect(in.Dst) {
					t.Errorf("%s moves %%%s into the other form", f.Name, name)
				}
				moved[in.Dst] = true
			}
		}
	}
	if !crossed {
		t.Errorf("%s: no move of %%%s crosses afterwards, so the shape is not pinned:\n%s", f.Name, name, p.Dump())
	}
}

// The closure's boxed parameter is moved on one branch and dropped where the branches join.
func TestPlanDirectMovesANamedLocalBeforeItCrosses(t *testing.T) {
	p := planned(t, moveClosureSrc)
	movesBeforeCrossing(t, p, funcNamed(t, p, "main.run.closure1"), "a")
}

// The profile specializes trySet; a key that is found is dropped at the join the inserting path moved it on.
func TestPlanDirectMovesTheKeyOfASpecializedTrySet(t *testing.T) {
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(moveMapKeySrc)))
	main.Files = []*syntax.Tree{main.Entry}
	mod := sem.Module{RecoverableOnly: true, IsrContracts: mir.IsrContracts}
	mod.Packages = append(testkit.LoadStd(t, "../../../std"), main)
	res := sem.CheckWith(&mod, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	p := mir.Build(res)
	err := p.Apply(
		mir.Transform{Name: "plan-rc", Run: mir.PlanRC},
		mir.Transform{Name: "specialize", Run: mir.Specialize},
		mir.Transform{Name: "elide-rc", Run: mir.ElideRC},
		mir.Transform{Name: "plan-direct", Run: mir.PlanDirect},
	)
	if err != nil {
		t.Fatalf("passes: %v", err)
	}
	movesBeforeCrossing(t, p, funcNamed(t, p, "std/map.Map.trySet[main.Id,Int]"), "key")
}
