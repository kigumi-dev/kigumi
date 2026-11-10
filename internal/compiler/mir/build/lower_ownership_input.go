package build

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	borrowpass "kigumi/internal/compiler/mir/ownership/borrow"
)

func newOwnershipInput(p *lowering) *ownquery.Input {
	return &ownquery.Input{
		R: p.R, ByEnt: p.ByEnt,
		// A borrow check can lower another body, so the function slice must stay live.
		Funcs:     func() []*midir.Func { return p.Funcs },
		Unlowered: p.Unlowered, HasBody: p.hasLowerableBody,
	}
}

// analyzeOwnership runs the move analysis and, on what it saw, the captured-read check, then drops the guard marks both read.
func analyzeOwnership(p *ownquery.Input, f *midir.Func, ghosts *ghostSet, record func(*midir.Func, []ownquery.MoveEvent)) {
	if moves, ok := ownquery.Analyze(p, f, ghosts.runs); ok {
		borrowpass.CheckReadCaptures(p, f, moves)
		if record != nil {
			record(f, moves)
		}
	}
	ownquery.ClearGuardMarks(f)
}
