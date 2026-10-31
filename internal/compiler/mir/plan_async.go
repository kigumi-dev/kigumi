package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
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

func planFrame(f *midir.Func, p *Program) {
	r := p.R
	live := liveAtSuspends(f)
	owned := owningLocals(f, r)
	parent := aliasParents(f)
	f.Slot = make([]int, len(f.Locals))
	for i := range f.Slot {
		f.Slot[i] = -1
	}
	f.Frame = 0
	f.Cleanup = make([][]midir.LocalID, f.Suspends+1)
	inFrame := map[midir.LocalID]bool{}
	for k, at := range live {
		var clean []midir.LocalID
		at.each(func(l midir.LocalID) {
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
		id := midir.LocalID(i)
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
func owningLocals(f *midir.Func, r *sem.Result) map[midir.LocalID]bool {
	owned := map[midir.LocalID]bool{}
	for i, l := range f.Locals {
		owned[midir.LocalID(i)] = !l.Borrowed && r.Types.Kind(l.Type) != sem.KRef
	}
	for _, prm := range f.Params {
		owned[prm] = true
	}
	defined := map[midir.LocalID]bool{}
	for _, prm := range f.Params {
		defined[prm] = true
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() {
				continue
			}
			defined[in.Dst] = true
			if in.Op == midir.OpBorrow || in.Op == midir.OpLendBox || (in.Op == midir.OpBuiltin && in.Str == "iter.at_ref") {
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
func aliasParents(f *midir.Func) map[midir.LocalID]midir.LocalID {
	parent := map[midir.LocalID]midir.LocalID{}
	defs := map[midir.LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() {
				continue
			}
			defs[in.Dst]++
			if (in.Op == midir.OpAlias || in.Op == midir.OpBorrow) && len(in.Args) == 1 {
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
