package mir_test

import (
	"testing"

	"kigumi/internal/mir"
)

const lentJoinReadSrc = `
type Point = {pub x Int, pub y Int}
type Loc = Point

fn pick(r: &Loc) -> &Loc { r }

fn named(r: &Loc, c: Bool) -> Int {
    let q = r
    (if c { q } else { r }).value.x
}

fn nested(r: &Loc, s: &Loc, c: Bool, d: Bool) -> Int {
    let q = s
    let v = (if c { (if d { q } else { r }) } else { s }).value
    v.y
}

fn arms(r: &Loc, s: &Loc, k: Int) -> Int {
    (match k {
        0 -> s,
        _ -> r,
    }).value.x
}

fn counted(r: &Loc, c: Bool) -> Int {
    let q = r
    (if c { q } else { pick(r) }).value.x
}

fn main() -> Void! {}
`

// aliasReleases counts the releases of a value read through an alias, as a defined type's .value is.
func aliasReleases(f *mir.Func) int {
	aliased := map[mir.LocalID]bool{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpAlias {
				aliased[in.Dst] = true
			}
		}
	}
	n := 0
	for l, k := range releasedLocals(f) {
		if aliased[l] {
			n += k
		}
	}
	return n
}

func TestPlanReleasesNothingReadThroughALentJoin(t *testing.T) {
	p := planned(t, lentJoinReadSrc)
	for _, name := range []string{"main.named", "main.nested", "main.arms"} {
		if n := aliasReleases(funcNamed(t, p, name)); n != 0 {
			t.Errorf("%s releases %d aliases, but every arm of its join is lent:\n%s", name, n, p.Dump())
		}
	}
	if aliasReleases(funcNamed(t, p, "main.counted")) != 1 {
		t.Errorf("main.counted does not release the count its join takes once:\n%s", p.Dump())
	}
}

func TestVerifyReleaseReadThroughALentCrossing(t *testing.T) {
	p := planned(t, lentJoinReadSrc)
	f := funcNamed(t, p, "main.named")
	for bi := range f.Blocks {
		blk := &f.Blocks[bi]
		for k, in := range blk.Insts {
			if in.Op != mir.OpUnboxDirect || !in.Lent || k+1 >= len(blk.Insts) || blk.Insts[k+1].Op != mir.OpAlias {
				continue
			}
			blk.Insts = append(blk.Insts, mir.Inst{Op: mir.OpRelease, Args: []mir.LocalID{blk.Insts[k+1].Dst}})
			wantRule(t, p, "droplent", "a reference this frame does not own")
			return
		}
	}
	t.Fatalf("main.named reads no lent crossing through an alias:\n%s", p.Dump())
}
