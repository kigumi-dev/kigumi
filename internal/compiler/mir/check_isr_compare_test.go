package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

func TestInterruptLoweringAcceptsComparisonBuiltins(t *testing.T) {
	prog := planned(t, isrPairSrc)
	tick := funcNamed(t, prog, "main.tick")
	flag := func() mir.LocalID {
		tick.Locals = append(tick.Locals, mir.Local{Type: sem.TyBool})
		return mir.LocalID(len(tick.Locals) - 1)
	}
	unordered, raw := flag(), flag()
	blk := &tick.Blocks[0]
	blk.Insts = append(blk.Insts,
		mir.Inst{Op: mir.OpBuiltin, Dst: unordered, Str: "cmp.unordered", Args: []mir.LocalID{raw, raw}},
		mir.Inst{Op: mir.OpBuiltin, Dst: flag(), Str: "cmp.guard", Args: []mir.LocalID{raw, unordered}},
	)
	if claims := mir.CheckInterruptLowering(prog); len(claims) != 0 {
		t.Fatalf("the NaN test and its guard read operands only: %s", claims[0].Diag.Msg)
	}
	blk.Insts = append(blk.Insts, mir.Inst{Op: mir.OpBuiltin, Dst: flag(), Str: "cmp.other", Args: []mir.LocalID{raw}})
	claims := mir.CheckInterruptLowering(prog)
	if len(claims) != 1 || !strings.Contains(claims[0].Diag.Msg, "calls the runtime helper `cmp.other`") {
		t.Fatalf("any other builtin stays a runtime helper: %+v", claims)
	}
}
