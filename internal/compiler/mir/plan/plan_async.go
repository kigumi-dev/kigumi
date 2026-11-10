package plan

import (
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	mirmobility "kigumi/internal/compiler/mir/mobility"
	mirunwind "kigumi/internal/compiler/mir/unwind"
)

func planAsync(p *planner) error {
	for _, f := range p.AllFuncs() {
		if !f.Async {
			continue
		}
		p.completeReturns(f)
		planFrame(f, p)
		mirmobility.Classify(f, p.R)
	}
	mirmobility.PinLentReads(p.Program)
	mirmobility.Refine(p.Program)
	return nil
}

func planFrame(f *midir.Func, p *planner) {
	r := p.R
	live := mirflow.LiveAtSuspends(f)
	owned := mirunwind.OwningLocals(f, r)
	parent := mirunwind.AliasParents(f)
	f.Slot = make([]int, len(f.Locals))
	for i := range f.Slot {
		f.Slot[i] = -1
	}
	f.Frame = 0
	f.Cleanup = make([][]midir.LocalID, f.Suspends+1)
	inFrame := map[midir.LocalID]bool{}
	for k, at := range live {
		var clean []midir.LocalID
		at.Each(func(l midir.LocalID) {
			inFrame[l] = true
			if src, ok := parent[l]; ok && at.Has(src) {
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
	for _, set := range mirunwind.Compute(f, r, true, nil, p.HeldBoxes(f)).Sets[1:] {
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
