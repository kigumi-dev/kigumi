package mir

import (
	"slices"

	"kigumi/internal/sem"
)

func planAsync(p *Program) error {
	for _, f := range p.allFuncs() {
		if !f.Async {
			continue
		}
		p.completeReturns(f)
		planFrame(f, p)
		classifyMobility(f, p.R)
	}
	refineMobility(p)
	return nil
}

func (p *Program) isAsyncBody(ent sem.EntityID) bool {
	e := p.R.Entity(ent)
	return e.Kind == sem.EntFn && p.R.Fn(ent).Declared&sem.EffAsync != 0
}

func planFrame(f *Func, p *Program) {
	r := p.R
	live := liveAtSuspends(f)
	owned := owningLocals(f, r)
	parent := aliasParents(f)
	f.Slot = make([]int, len(f.Locals))
	for i := range f.Slot {
		f.Slot[i] = -1
	}
	f.Frame = 0
	f.Cleanup = make([][]LocalID, f.Suspends+1)
	inFrame := map[LocalID]bool{}
	for k, at := range live {
		var clean []LocalID
		at.each(func(l LocalID) {
			inFrame[l] = true
			if src, ok := parent[l]; ok && at.has(src) {
				return
			}
			if owned[l] {
				clean = append(clean, l)
			}
		})
		slices.Sort(clean)
		slices.Reverse(clean)
		f.Cleanup[k] = clean
	}
	for _, set := range planUnwind(f, r, true, nil, p.heldBoxes(f)).Sets[1:] {
		for _, l := range set {
			inFrame[l] = true
		}
	}
	f.PanicDrop = nil
	for i, l := range f.Locals {
		id := LocalID(i)
		named := owned[id] && l.Ent != 0
		if named {
			inFrame[id] = true
		}
		if inFrame[id] {
			f.Slot[i] = f.Frame
			f.Frame++
		}
		if named {
			if src, ok := parent[id]; !ok || !owned[src] || f.Locals[src].Ent == 0 {
				f.PanicDrop = append(f.PanicDrop, id)
			}
		}
	}
	slices.Reverse(f.PanicDrop)
}

// owningLocals marks the locals whose slot carries a reference: not a lent
// receiver, not a lease, and not a borrow of something else.
func owningLocals(f *Func, r *sem.Result) map[LocalID]bool {
	owned := map[LocalID]bool{}
	for i, l := range f.Locals {
		owned[LocalID(i)] = !l.Borrowed && r.Types.Kind(l.Type) != sem.KRef
	}
	for _, prm := range f.Params {
		owned[prm] = true
	}
	defined := map[LocalID]bool{}
	for _, prm := range f.Params {
		defined[prm] = true
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !shapes[in.Op].result {
				continue
			}
			defined[in.Dst] = true
			if in.Op == OpBorrow || in.Op == OpLendBox || (in.Op == OpBuiltin && in.Str == "iter.at_ref") {
				owned[in.Dst] = false
			}
		}
	}
	for l := range owned {
		if !defined[l] {
			owned[l] = false
		}
	}
	return owned
}

// aliasParents maps a local to the one whose reference it stands for; the
// two share a reference, so an abandoned frame releases it once.
func aliasParents(f *Func) map[LocalID]LocalID {
	parent := map[LocalID]LocalID{}
	defs := map[LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !shapes[in.Op].result {
				continue
			}
			defs[in.Dst]++
			if (in.Op == OpAlias || in.Op == OpBorrow) && len(in.Args) == 1 {
				parent[in.Dst] = in.Args[0]
			}
		}
	}
	for l, n := range defs {
		if n > 1 {
			delete(parent, l)
		}
	}
	return parent
}
