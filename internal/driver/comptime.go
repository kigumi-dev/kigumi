package driver

import (
	"fmt"

	"kigumi/internal/diag"
	"kigumi/internal/mir"
	"kigumi/internal/vm"
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
		f := &res.Files[ce.Site.File]
		tree := res.Tree(ce.Site.File)
		f.Diags = append(f.Diags, diag.Errorf(diag.At(tree.File, tree.Span(ce.Site.Node)), "comptime: "+ce.Err.Error()))
	}
	return nil
}
