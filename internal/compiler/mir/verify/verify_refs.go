package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type refFlow struct {
	weak                []bool
	f                   *midir.Func
	site                []int32
	siteBase            []int
	nobj                int
	loop                []bool
	live                *edgeLive
	track               []bool
	counted             []bool
	undef, nulled, lent int32
}

func (p *verifier) verifyRefs(f *midir.Func) *Error {
	if f.RCBase == 0 || len(f.Blocks) == 0 || !hasRelease(f) {
		return nil
	}
	rf := p.newRefFlow(f)
	in := make([]*refState, len(f.Blocks))
	in[0] = rf.entry(p)
	var found *Error
	acyclic := solveForward(f, func(bid midir.BlockID, wake func(midir.BlockID), final bool) bool {
		out, err := rf.block(p, bid, in[bid], final)
		if err != nil {
			found = err
			return false
		}
		if diverges(f, &f.Blocks[bid]) {
			return true
		}
		rf.live.dead(f, bid, func(l midir.LocalID) { out.ptr.set(l, rf.undef) })
		targets := f.Blocks[bid].Term.Targets
		for k, succ := range targets {
			switch {
			case in[succ] == nil:
				if k == len(targets)-1 && final {
					in[succ] = out
				} else {
					in[succ] = out.clone()
				}
				wake(succ)
			case in[succ].join(out):
				wake(succ)
			}
		}
		return true
	})
	if found != nil {
		return found
	}
	if acyclic {
		return nil
	}
	for bid := range f.Blocks {
		if in[bid] == nil {
			continue
		}
		if _, err := rf.block(p, midir.BlockID(bid), in[bid], true); err != nil {
			return err
		}
	}
	return nil
}

func hasRelease(f *midir.Func) bool {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpDrop || in.Op == midir.OpRelease || in.Op == midir.OpMove {
				return true
			}
		}
	}
	return false
}

func (p *verifier) newRefFlow(f *midir.Func) *refFlow {
	rf := &refFlow{f: f, weak: p.body(f).weakDefs, loop: cyclicBlocks(f), nobj: len(f.Params), siteBase: make([]int, len(f.Blocks)), track: make([]bool, len(f.Locals)), counted: make([]bool, len(f.Locals))}
	rf.live = p.edgeLive(f)
	lent := p.unownedRefs(f)
	for l := range f.Locals {
		t := f.Locals[l].Type
		rf.track[l] = true
		rf.counted[l] = !f.Locals[l].Borrowed && !lent[l] && t != sem.TyUnit && t != sem.TyNever && !p.R.RegScalar(t)
	}
	total := 0
	for bid, blk := range f.Blocks {
		rf.siteBase[bid] = total
		total += len(blk.Insts)
	}
	rf.site = make([]int32, total)
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			rf.site[rf.siteBase[bid]+i] = -1
			if in.Op.Yields() && rf.counted[in.Dst] {
				rf.site[rf.siteBase[bid]+i] = int32(rf.nobj)
				rf.nobj++
			}
		}
	}
	return rf
}

func (rf *refFlow) entry(p *verifier) *refState {
	f := rf.f
	sets := newObjSets(3*rf.nobj, len(f.Locals))
	rf.undef, rf.nulled, rf.lent = sets.of(refUndef), sets.of(refNulled), sets.of(refLent)
	s := &refState{sets: sets, ptr: newPtrVec(len(f.Locals), sets, rf.undef)}
	for i, prm := range f.Params {
		switch {
		case rf.counted[prm]:
			s.ptr.set(prm, s.sets.single(int32(i)*3))
		case rf.track[prm]:
			s.ptr.set(prm, rf.lent)
		}
	}
	return s
}
