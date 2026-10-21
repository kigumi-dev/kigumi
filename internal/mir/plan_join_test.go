package mir_test

import (
	"testing"

	"kigumi/internal/mir"
)

func releasedLocals(f *mir.Func) map[mir.LocalID]int {
	out := map[mir.LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpRelease {
				out[in.Args[0]]++
			}
		}
	}
	return out
}

func TestPlanReleasesATemporaryReadInTheBlockWhereBranchesJoin(t *testing.T) {
	p := planned(t, `
fn pick(v: Int) -> Bool {
    let n = (v.toUsize() || 0) + 1
    n > 3
}

fn main() -> Void! {}
`)
	f := funcNamed(t, p, "main.pick")
	released := releasedLocals(f)
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpAlias && f.Locals[in.Dst].Ent == 0 && f.Locals[in.Args[0]].Ent == 0 && released[in.Dst] != 1 {
				t.Errorf("the merged temporary %%%d is released %d times:\n%s", in.Dst, released[in.Dst], p.Dump())
			}
		}
	}
}

func TestPlanReleasesTheIndexOfASetIndexAfterIt(t *testing.T) {
	p := planned(t, `
fn put() {
    let mut a = FixedArray.filled[Int, 3](0)
    a[1] = 5
}

fn main() -> Void! {}
`)
	f := funcNamed(t, p, "main.put")
	released := releasedLocals(f)
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpSetIndex && f.Locals[in.Args[1]].Ent == 0 && released[in.Args[1]] == 0 {
				t.Errorf("the index %%%d of the store is never released:\n%s", in.Args[1], p.Dump())
			}
		}
	}
}

func TestSpecializeDropsTheWitnessBindsOfAnInlinedDerivedEquals(t *testing.T) {
	p := specialized(t, `
fn same(a: Int?, b: Int?) -> Bool { a == b }

fn fingerprint(a: Int?) -> u64 { hash(&a) }

fn main() -> Void! {}
`)
	same := funcNamed(t, p, "main.same")
	if n := countOp(same, mir.OpBind) + countOp(same, mir.OpClosure) + countOp(same, mir.OpFnItem); n != 0 {
		t.Errorf("main.same keeps %d witness values nothing receives:\n%s", n, p.Dump())
	}
	fingerprint := funcNamed(t, p, "main.fingerprint")
	released := releasedLocals(fingerprint)
	for _, blk := range fingerprint.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpBind && released[in.Dst] == 0 {
				t.Errorf("main.fingerprint binds %%%d and never releases it:\n%s", in.Dst, p.Dump())
			}
		}
	}
}
