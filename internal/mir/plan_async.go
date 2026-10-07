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
		completeReturns(f)
		planFrame(f, p.R)
		classifyMobility(f, p.R)
	}
	refineMobility(p)
	return nil
}

func (p *Program) isAsyncBody(ent sem.EntityID) bool {
	e := p.R.Entity(ent)
	return e.Kind == sem.EntFn && p.R.Fn(ent).Declared&sem.EffAsync != 0
}

func completeReturns(f *Func) {
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		if blk.Term.Op != TermReturn {
			continue
		}
		blk.Insts = append(blk.Insts, Inst{Op: OpFutureDone, Args: []LocalID{blk.Term.Args[0]}})
	}
}

func planFrame(f *Func, r *sem.Result) {
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
		for l := range at {
			inFrame[l] = true
			if src, ok := parent[l]; ok && at[src] {
				continue
			}
			if owned[l] {
				clean = append(clean, l)
			}
		}
		slices.Sort(clean)
		slices.Reverse(clean)
		f.Cleanup[k] = clean
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

// liveAtSuspends is a backward liveness over every local; entry k holds
// what is live right after the suspension point numbered k.
func liveAtSuspends(f *Func) []map[LocalID]bool {
	n := len(f.Blocks)
	liveIn := make([]map[LocalID]bool, n)
	for i := range liveIn {
		liveIn[i] = map[LocalID]bool{}
	}
	at := make([]map[LocalID]bool, f.Suspends+1)
	for i := range at {
		at[i] = map[LocalID]bool{}
	}
	for changed := true; changed; {
		changed = false
		for bid := n - 1; bid >= 0; bid-- {
			blk := &f.Blocks[bid]
			live := map[LocalID]bool{}
			for _, t := range blk.Term.Targets {
				for l := range liveIn[t] {
					live[l] = true
				}
			}
			for _, a := range blk.Term.Args {
				live[a] = true
			}
			for i := len(blk.Insts) - 1; i >= 0; i-- {
				in := blk.Insts[i]
				if in.Op == OpAwait && in.Index < len(at) {
					for l := range live {
						at[in.Index][l] = true
					}
				}
				if shapes[in.Op].result {
					delete(live, in.Dst)
				}
				for _, a := range in.Args {
					live[a] = true
				}
			}
			if !sameSet(live, liveIn[bid]) {
				liveIn[bid] = live
				changed = true
			}
		}
	}
	return at
}

// owningLocals marks the locals whose slot carries a reference: not a lent
// receiver, not a lease, and not a borrow of something else.
func owningLocals(f *Func, r *sem.Result) map[LocalID]bool {
	owned := map[LocalID]bool{}
	for i, l := range f.Locals {
		owned[LocalID(i)] = !l.Borrowed && r.Types.Kind(l.Type) != sem.KRef
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
			if in.Op == OpBorrow || (in.Op == OpBuiltin && in.Str == "iter.at_ref") {
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
