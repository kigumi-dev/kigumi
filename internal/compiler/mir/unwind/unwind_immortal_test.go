package unwind_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	mirunwind "kigumi/internal/compiler/mir/unwind"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
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
	prog := mirfixture.Planned(t, immortalSrc)
	f := mirfixture.FuncNamed(t, prog, "main.work")
	imm := mirunwind.ImmortalLocals(f, prog.R)
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
		if imm[midir.LocalID(l)] {
			t.Errorf("parameter %d is immortal", l)
		}
	}
}

func definedOnlyByConst(f *midir.Func, l midir.LocalID) bool {
	seen := false
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Dst != l {
				continue
			}
			switch in.Op {
			case midir.OpConst, midir.OpUnit:
				seen = true
			case midir.OpDrop, midir.OpRelease, midir.OpPanic, midir.OpNop, midir.OpSetField, midir.OpSetIndex, midir.OpCellSet:
			default:
				return false
			}
		}
	}
	return seen
}
