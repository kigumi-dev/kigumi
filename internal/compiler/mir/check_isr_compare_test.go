package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func TestInterruptLoweringAcceptsComparisonBuiltins(t *testing.T) {
	prog := planned(t, isrPairSrc)
	tick := funcNamed(t, prog, "main.tick")
	flag := func() midir.LocalID {
		tick.Locals = append(tick.Locals, midir.Local{Type: sem.TyBool})
		return midir.LocalID(len(tick.Locals) - 1)
	}
	unordered, raw := flag(), flag()
	blk := &tick.Blocks[0]
	blk.Insts = append(blk.Insts,
		midir.Inst{Op: midir.OpBuiltin, Dst: unordered, Str: "cmp.unordered", Args: []midir.LocalID{raw, raw}},
		midir.Inst{Op: midir.OpBuiltin, Dst: flag(), Str: "cmp.guard", Args: []midir.LocalID{raw, unordered}},
	)
	if claims := mir.CheckInterruptLowering(prog); len(claims) != 0 {
		t.Fatalf("the NaN test and its guard read operands only: %s", claims[0].Diag.Msg)
	}
	blk.Insts = append(blk.Insts, midir.Inst{Op: midir.OpBuiltin, Dst: flag(), Str: "cmp.other", Args: []midir.LocalID{raw}})
	claims := mir.CheckInterruptLowering(prog)
	if len(claims) != 1 || !strings.Contains(claims[0].Diag.Msg, "calls the runtime helper `cmp.other`") {
		t.Fatalf("any other builtin stays a runtime helper: %+v", claims)
	}
}
