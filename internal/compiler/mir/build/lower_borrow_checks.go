package build

import (
	midir "kigumi/internal/compiler/mir/ir"
	borrowpass "kigumi/internal/compiler/mir/ownership/borrow"
	"kigumi/internal/compiler/sem"
)

// pendingCheck is a body whose borrow checks wait for the end of lowering, with the top-level body it was lowered under.
type pendingCheck struct {
	f   *midir.Func
	cur sem.EntityID
}

// checkBorrowsOf defers f's borrow checks to the end of lowering, so storesParam, which reads p.ByEnt, sees every
// lowered body; what goes through calleeBody still depends on entity order, as runPendingChecks keeps it.
func (p *lowering) checkBorrowsOf(f *midir.Func) {
	p.checks = append(p.checks, pendingCheck{f: f, cur: p.cur})
}

func (p *lowering) runBorrowChecks(f *midir.Func) {
	if p.borrow == nil {
		p.borrow = borrowpass.New(p.input, p.calleeBody)
	}
	p.borrow.Check(f)
}

// Each check sees the bodies available at its original lowering position.
func (p *lowering) runPendingChecks() {
	saved := p.cur
	for i := 0; i < len(p.checks); i++ {
		p.cur = p.checks[i].cur
		p.runBorrowChecks(p.checks[i].f)
	}
	p.checks, p.cur = nil, saved
}
