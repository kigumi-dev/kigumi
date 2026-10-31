package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// An awaited operand parks in the awaiter's own frame until it is ready, so
// whatever pins it pins the awaiter; the fixpoint follows that to a stop.
func refineMobility(p *Program) {
	for changed := true; changed; {
		changed = false
		for _, f := range p.allFuncs() {
			if !f.Async || p.R.Entity(f.Ent).Kind != sem.EntFn {
				continue
			}
			info := p.R.Fn(f.Ent)
			if info.Mobility != sem.MobilityMigratable {
				continue
			}
			if by, pinned := awaitedPin(p, f); pinned {
				info.Mobility, info.BoundBy = sem.MobilityExecutorBound, by
				changed = true
			}
		}
	}
}

// awaitedPin names what a resume on another worker would have to carry: the
// callee's own reason where the body is known, the operand's type otherwise.
func awaitedPin(p *Program, f *midir.Func) (string, bool) {
	t := newFutureTrace()
	yields := map[midir.LocalID]bool{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			t.note(in)
			if in.Op == midir.OpCall && p.R.IsTaskYieldNow(in.Ent) {
				yields[in.Dst] = true
			}
			if in.Op != midir.OpAwait || len(in.Args) != 1 || yields[t.origin(in.Args[0])] {
				continue
			}
			if ent := t.asyncBody(in.Args[0]); ent != 0 {
				if callee := p.R.Fn(ent); callee.Mobility == sem.MobilityExecutorBound {
					return callee.BoundBy, true
				}
				continue
			}
			if ty := f.Locals[in.Args[0]].Type; !p.R.IsSend(ty) {
				return p.R.TypeString(ty), true
			}
		}
	}
	return "", false
}
