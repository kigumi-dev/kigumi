package vm

import (
	"fmt"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

// checkComptimeLiterals rejects an integer literal of the block that its type cannot hold, which the checker could not see while the block's type was still open.
func checkComptimeLiterals(prog *mir.Program, f *midir.Func) error {
	for _, b := range f.Blocks {
		for _, in := range b.Insts {
			if in.Op == midir.OpConst && !prog.R.LiteralFits(in.Lit, in.Type) {
				return fmt.Errorf("`%s` does not fit `%s`", in.Lit.Int, prog.R.TypeString(in.Type))
			}
		}
	}
	return nil
}
