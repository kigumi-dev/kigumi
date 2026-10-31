package mir_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
)

const captureSrc = `
type Pt = {
    pub x Int
}

fn Pt.sum(self, k: Int) -> Int { self.x + k }

fn nested(p: Pt) -> Int {
    let g = (k: Int) => p.sum(k)
    let h = (k: Int) => g(k + 1)
    h(1)
}

fn main() -> Void! {}
`

func TestPlanReleasesTheCopyAClosureRetains(t *testing.T) {
	p := planned(t, captureSrc)
	f := funcNamed(t, p, "main.nested")
	released := map[mir.LocalID]bool{}
	var shares []mir.LocalID
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			switch in.Op {
			case mir.OpRelease:
				released[in.Args[0]] = true
			case mir.OpShare:
				shares = append(shares, in.Dst)
			}
		}
	}
	if len(shares) == 0 {
		t.Fatalf("nested plans no share for a capture:\n%s", p.Dump())
	}
	for _, s := range shares {
		if !released[s] {
			t.Errorf("share %%%s is never released although the closure retains its own:\n%s", f.Locals[s].Name, p.Dump())
		}
	}
}
