package mir

import (
	"sync"

	midir "kigumi/internal/compiler/mir/ir"
	mirunwind "kigumi/internal/compiler/mir/unwind"
	"kigumi/internal/compiler/sem"
)

type unwindMemo struct {
	mu    sync.Mutex
	plans map[*midir.Func]*mirunwind.Plan
}

func (p *Program) UnwindPlanOf(f *midir.Func) *mirunwind.Plan {
	p.unwind.mu.Lock()
	defer p.unwind.mu.Unlock()
	if plan, ok := p.unwind.plans[f]; ok {
		return plan
	}
	if p.unwind.plans == nil {
		p.unwind.plans = map[*midir.Func]*mirunwind.Plan{}
	}
	plan := mirunwind.Compute(f, p.R, false, p.lentParamLocals(f), p.HeldBoxes(f))
	p.unwind.plans[f] = plan
	return plan
}

// A parameter the body only borrows stays with the caller, whose own frame releases it.
func (p *Program) lentParamLocals(f *midir.Func) map[midir.LocalID]bool {
	if f.Async {
		return nil
	}
	lent := map[midir.LocalID]bool{}
	isFn := f.Ent != 0 && p.R.Entity(f.Ent).Kind == sem.EntFn
	for i, prm := range f.Params {
		if isFn && p.ParamLent(f.Ent, i) || p.CaptureLent(f, i) {
			lent[prm] = true
		}
	}
	return lent
}
