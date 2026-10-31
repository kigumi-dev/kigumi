package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// lateReleases are resource temporaries released at their full expression's end, latest created first.
type lateReleases map[Point][]LocalID

// releasedLate leaves out a temporary its last use hands to a callee or a
// closure by value: the drop goes with it, even where the callee only lends.
func (p *Program) releasedLate(f *Func, t LocalID, u use) bool {
	if !p.R.HoldsResource(f.Locals[t].Type) {
		return false
	}
	insts := f.Blocks[u.block].Insts
	return u.inst >= len(insts) || !p.passedByValue(f, insts[u.inst], t)
}

func (p *Program) passedByValue(f *Func, in Inst, t LocalID) bool {
	switch in.Op {
	case OpClosure, OpBind, OpFutureNew, OpCallC:
		return slices.Contains(in.Args, t)
	case OpCall, OpCallValue:
	default:
		return false
	}
	for i, a := range in.Args {
		if a != t || in.Op == OpCallValue && i == 0 || p.lentReceiver(in, i) || p.lentByType(f, &in, i) {
			continue
		}
		if pt, ok := p.declaredParam(in, i); ok && p.R.Types.Kind(pt) == sem.KRef {
			continue
		}
		return true
	}
	return false
}

func (p *Program) lentReceiver(in Inst, i int) bool {
	at := 0
	if in.Op == OpCallValue {
		at = 1
	}
	if i != at || in.Ent == 0 || p.R.Entity(in.Ent).Kind != sem.EntFn {
		return false
	}
	recv := p.R.Fn(in.Ent).Recv
	return recv == sem.RecvSelf || recv == sem.RecvMut
}

func (l lateReleases) add(f *Func, t LocalID, u use) {
	at := Point{u.block, fullEnd(f, f.Locals[t].FullExpr, u.block, u.inst)}
	l[at] = append(l[at], t)
}

func (l lateReleases) into(plan map[Point][]LocalID) {
	for at, ts := range l {
		slices.Sort(ts)
		slices.Reverse(ts)
		plan[at] = append(plan[at], ts...)
	}
}

// fullEnd stops at an await: the async frame saves only what is read after it.
func fullEnd(f *Func, fx int32, b, k int) int {
	if fx == 0 {
		return k
	}
	insts := f.Blocks[b].Insts
	for i := k + 1; i < len(insts); i++ {
		switch {
		case insts[i].Op == OpAwait:
			return k
		case insts[i].FullEnd == fx:
			return i
		}
	}
	return k
}
