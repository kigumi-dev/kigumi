package mir

import "kigumi/internal/sem"

// pendingCheck is a body whose borrow checks wait for the end of lowering, with the top-level body it was lowered under.
type pendingCheck struct {
	f   *Func
	cur sem.EntityID
}

// checkBorrowsOf defers f's borrow checks until every reachable body is lowered, so no answer depends on declaration order.
func (p *Program) checkBorrowsOf(f *Func) {
	if l := p.lowering; l != nil {
		l.checks = append(l.checks, pendingCheck{f: f, cur: l.cur})
		return
	}
	runBorrowChecks(p, f)
}

func runBorrowChecks(p *Program, f *Func) {
	analyzeBorrows(p, f)
	checkBorrowedMoves(p, f)
	checkImmediateCaptures(p, f)
	checkErasedReaders(p, f)
}

// runPendingChecks restores each body's lowering position, so calleeBody answers as it did right after that body was built.
func (p *Program) runPendingChecks() {
	l := p.lowering
	saved := l.cur
	for i := 0; i < len(l.checks); i++ {
		l.cur = l.checks[i].cur
		runBorrowChecks(p, l.checks[i].f)
	}
	l.checks, l.cur = nil, saved
}
