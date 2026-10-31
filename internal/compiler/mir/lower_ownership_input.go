package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
)

func newOwnershipInput(p *Program) *ownquery.Input {
	return &ownquery.Input{
		R: p.R, ByEnt: p.ByEnt,
		// A borrow check can lower another body, so the function slice must stay live.
		Funcs:     func() []*midir.Func { return p.Funcs },
		Unlowered: p.Unlowered, HasBody: p.hasLowerableBody,
	}
}
