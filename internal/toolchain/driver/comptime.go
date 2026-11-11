package driver

import (
	"fmt"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/mir"
)

// foldComptime evaluates the program's comptime blocks in the VM's sandbox
// and folds the results into the MIR; a failure becomes a diagnostic at
// the block.
func foldComptime(prog *mir.Program) error {
	for _, cf := range prog.Comptime {
		if !cf.Func.DirectPlanned() {
			return fmt.Errorf("%s would run before plan-direct, but a comptime block runs the planned MIR a run does", cf.Func.Name)
		}
	}
	res := prog.R
	for _, ce := range vm.FoldComptime(prog) {
		res.AppendDiag(ce.Site.File, comptimeDiag(res, ce))
	}
	return nil
}
