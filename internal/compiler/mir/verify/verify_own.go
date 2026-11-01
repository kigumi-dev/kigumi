package verify

import (
	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// Ownership states of a move-only named local for the verifier: it holds
// nothing, holds a value, or holds a value on some paths only (a pattern
// binding behind `&&`).
type ownVerify uint8

const (
	ovNone ownVerify = iota
	ovOwned
	ovMaybe
)

// verifyOwnership proves exactly-once ownership of every move-only named
// local: a value is consumed (moved, handed to an owning callee, or
// dropped) exactly once on every path, nothing is read after it is
// consumed, no live value is overwritten, and nothing is still owned when
// the function returns. Cells, borrows, lent receivers and a destructor's
// own `self` are outside the rule; temporaries follow the runtime plan.
func (p *verifier) verifyOwnership(f *midir.Func) *Error {
	tracked := make([]bool, len(f.Locals))
	any := false
	dtorSelf, isDtor := p.destructorSelf(f)
	for i, l := range f.Locals {
		if l.Ent != 0 && !l.Cell && !l.Borrowed && !(isDtor && midir.LocalID(i) == dtorSelf) && !p.R.IsCopy(l.Type) && p.R.Types.Kind(l.Type) != sem.KRef {
			tracked[i], any = true, true
		}
	}
	if !any {
		return nil
	}
	in := make([]*ownquery.CowVec[ownVerify], len(f.Blocks))
	entry := &ownquery.CowVec[ownVerify]{}
	for _, prm := range f.Params {
		if tracked[prm] {
			entry.Set(prm, ovOwned)
		}
	}
	in[0] = entry
	// The states settle first; the rules are checked once they have, so a
	// join that a later predecessor widens is not reported early.
	work := []midir.BlockID{0}
	thread := mirflow.NewFlagThread(f)
	for len(work) > 0 {
		bid := work[0]
		work = work[1:]
		out, _ := p.transferOwn(f, bid, in[bid], tracked, false)
		if diverges(f, &f.Blocks[bid]) {
			continue
		}
		if thread.AllThreaded(bid) {
			continue
		}
		for _, succ := range f.Blocks[bid].Term.Targets {
			if joinVerifyInto(in, succ, out) {
				work = append(work, succ)
			}
			if to, ok := thread.Target(bid, succ); ok {
				if thru, _ := p.transferOwn(f, succ, out, tracked, false); joinVerifyInto(in, to, thru) {
					work = append(work, to)
				}
			}
		}
	}
	for bid := range f.Blocks {
		if in[bid] == nil {
			continue
		}
		if _, err := p.transferOwn(f, midir.BlockID(bid), in[bid], tracked, true); err != nil {
			return err
		}
	}
	return nil
}

// destructorSelf is the `move self` of a destructor, which the runtime is
// already destroying: the body neither drops nor moves it.
func (p *verifier) destructorSelf(f *midir.Func) (midir.LocalID, bool) {
	if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntFn {
		return 0, false
	}
	info := p.R.Fn(f.Ent)
	if info.Owner == 0 || p.R.Entity(info.Owner).Kind != sem.EntType || p.R.TypeDecl(info.Owner).Drop != f.Ent || len(f.Params) == 0 {
		return 0, false
	}
	return f.Params[0], true
}

func joinVerifyInto(in []*ownquery.CowVec[ownVerify], b midir.BlockID, out *ownquery.CowVec[ownVerify]) bool {
	if in[b] == nil {
		cp := out.Clone()
		in[b] = &cp
		return true
	}
	return in[b].Join(out, joinVerify)
}

// joinVerify merges the states of two predecessors: a value on one side
// only is a value on some paths.
func joinVerify(a, b ownVerify) ownVerify {
	if a == b {
		return a
	}
	return ovMaybe
}

// transferOwn walks a block from its entry state; in check mode it also
// reports the first broken rule.
func (p *verifier) transferOwn(f *midir.Func, bid midir.BlockID, entry *ownquery.CowVec[ownVerify], tracked []bool, check bool) (*ownquery.CowVec[ownVerify], *Error) {
	cp := entry.Clone()
	st := &cp
	// A plain copy of a named local into a temporary aliases it (the plan
	// takes no reference); dropping that temporary releases the local.
	alias := map[midir.LocalID]midir.LocalID{}
	crossed := map[midir.LocalID]midir.LocalID{}
	blk := &f.Blocks[bid]
	broken := func(i int, format string, args ...any) (*ownquery.CowVec[ownVerify], *Error) {
		if check {
			return nil, fail("ownership", bid, i, format, args...)
		}
		return st, nil
	}
	for i := range blk.Insts {
		in := &blk.Insts[i]
		switch in.Op {
		case midir.OpNop:
			continue
		case midir.OpDrop:
			l := in.Args[0]
			if src, ok := alias[l]; ok {
				l = src
			}
			if !tracked[l] {
				continue
			}
			if st.Get(l) == ovNone && check {
				return broken(i, "drop of %%%s, which holds nothing", f.Locals[l].Name)
			}
			st.Set(l, ovNone)
			continue
		}
		for k, a := range in.Args {
			if !tracked[a] {
				continue
			}
			if st.Get(a) == ovNone && check {
				return broken(i, "%s reads %%%s after it was consumed", in.Op, f.Locals[a].Name)
			}
			if !in.Rest && (in.Op == midir.OpMove || ownquery.ConsumesArg(p.R, p.ByEnt, in, k)) {
				st.Set(a, ovNone)
			}
		}
		if src, ok := movedCross(in, crossed); ok {
			st.Set(src, ovNone)
		}
		if src, ok := p.countedCrossSource(f, in, tracked); ok {
			crossed[in.Dst] = src
		}
		if in.Op == midir.OpCopy && !in.Share && f.Locals[in.Dst].Ent == 0 && tracked[in.Args[0]] {
			alias[in.Dst] = in.Args[0]
		}
		if in.Op == midir.OpAlias && f.Locals[in.Dst].Ent == 0 && tracked[in.Args[0]] {
			alias[in.Dst] = in.Args[0]
		}
		if in.Op.Yields() && tracked[in.Dst] {
			if st.Get(in.Dst) == ovOwned && check {
				return broken(i, "%s overwrites %%%s while it still holds a value", in.Op, f.Locals[in.Dst].Name)
			}
			st.Set(in.Dst, ovOwned)
		}
	}
	if !check {
		return st, nil
	}
	for _, a := range blk.Term.Args {
		if tracked[a] && st.Get(a) == ovNone {
			return broken(len(blk.Insts), "terminator reads %%%s after it was consumed", f.Locals[a].Name)
		}
	}
	if blk.Term.Op == midir.TermReturn && !diverges(f, blk) {
		var left midir.LocalID
		found := false
		st.Each(func(l midir.LocalID, _ ownVerify) {
			if tracked[l] && !found {
				left, found = l, true
			}
		})
		if found {
			return broken(len(blk.Insts), "%%%s is still owned at return", f.Locals[left].Name)
		}
	}
	return st, nil
}
