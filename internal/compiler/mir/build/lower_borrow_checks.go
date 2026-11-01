package build

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	borrowpass "kigumi/internal/compiler/mir/ownership/borrow"
	"kigumi/internal/compiler/sem"
)

func newLowering(r *sem.Result, keep func(path string) bool) *lowering {
	l := &lowering{Program: &mir.Program{R: r, ByEnt: map[sem.EntityID]*midir.Func{}}, rootKeep: keep}
	l.input = newOwnershipInput(l)
	l.borrow = borrowpass.New(l.input, l.calleeBody)
	return l
}

func lowerChecked(r *sem.Result, all bool, keep func(path string) bool) *lowering {
	l := newLowering(r, keep)
	l.lowerReachable(all)
	l.Diags = append(l.Diags, l.borrow.CheckRefScopes()...)
	return l
}

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

// Each check sees the bodies available at its original lowering position.
func (p *lowering) runPendingChecks() {
	saved := p.cur
	for i := 0; i < len(p.checks); i++ {
		p.cur = p.checks[i].cur
		p.borrow.Check(p.checks[i].f)
	}
	p.checks, p.cur = nil, saved
}
