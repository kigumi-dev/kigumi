package mir_test

import (
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

const immortalSrc = `
fn stop() -> Never {
    for {}
}

fn work(n: Int) -> Int {
    let label = "text"
    let k = 7
    if n > 100 {
        stop()
    }
    k + n + label.len().toInt().unwrap()
}

print "${work(1)}"
`

func TestImmortalLocalsAreConstantsAndNever(t *testing.T) {
	prog := planned(t, immortalSrc)
	f := funcNamed(t, prog, "main.work")
	imm := mir.ImmortalLocals(f, prog.R)
	consts, nevers := 0, 0
	for l := range imm {
		switch {
		case f.Locals[l].Type == sem.TyNever:
			nevers++
		case f.Locals[l].Type != sem.TyUnit:
			if !definedOnlyByConst(f, l) {
				t.Errorf("local %d (%s) is immortal but is defined by more than a constant", l, prog.R.TypeString(f.Locals[l].Type))
			}
			consts++
		}
	}
	if consts == 0 {
		t.Error("no constant-only local is immortal")
	}
	if nevers == 0 {
		t.Error("no never-typed local is immortal")
	}
	for l := range f.Params {
		if imm[mir.LocalID(l)] {
			t.Errorf("parameter %d is immortal", l)
		}
	}
}

func definedOnlyByConst(f *mir.Func, l mir.LocalID) bool {
	seen := false
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Dst != l {
				continue
			}
			switch in.Op {
			case mir.OpConst, mir.OpUnit:
				seen = true
			case mir.OpDrop, mir.OpRelease, mir.OpPanic, mir.OpNop, mir.OpSetField, mir.OpSetIndex, mir.OpCellSet:
			default:
				return false
			}
		}
	}
	return seen
}
