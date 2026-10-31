package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

func (rf *refFlow) block(p *verifier, bid midir.BlockID, entry *refState, check bool) (*refState, *Error) {
	s := entry
	if !check {
		s = entry.clone()
	}
	f := rf.f
	blk := &f.Blocks[bid]
	broken := func(i int, format string, args ...any) *Error {
		if check {
			return fail("refs", bid, i, format, args...)
		}
		return nil
	}
	for i := range blk.Insts {
		in := &blk.Insts[i]
		if in.Op != midir.OpDrop && in.Op != midir.OpRelease {
			for _, a := range in.Args {
				if rf.track[a] && s.gone(a) {
					if err := broken(i, "%s reads %%%s after its value was released or moved out", in.Op, f.Locals[a].Name); err != nil {
						return nil, err
					}
				}
			}
		}
		if err := rf.step(p, s, bid, i, in, broken); err != nil {
			return nil, err
		}
	}
	for _, a := range blk.Term.Args {
		if rf.track[a] && s.gone(a) {
			if err := broken(len(blk.Insts), "terminator reads %%%s after its value was released or moved out", f.Locals[a].Name); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}

func (rf *refFlow) step(p *verifier, s *refState, bid midir.BlockID, i int, in *midir.Inst, broken func(int, string, ...any) *Error) *Error {
	f := rf.f
	switch in.Op {
	case midir.OpDrop, midir.OpRelease:
		a := in.Args[0]
		if !rf.track[a] {
			return nil
		}
		named := f.Locals[a].Ent != 0
		nulled := s.ptr.get(a) == rf.nulled
		if !(named && nulled) && rf.counted[a] && s.release(a) {
			if err := broken(i, "%s of %%%s, whose reference is already released", in.Op, f.Locals[a].Name); err != nil {
				return err
			}
		}
		if in.Op == midir.OpDrop && named {
			s.ptr.set(a, rf.nulled)
		}
		return nil
	}
	for k, a := range in.Args {
		if rf.counted[a] && p.takesForGood(f, in, k) {
			s.release(a)
		}
	}
	if !in.Op.Yields() || !rf.track[in.Dst] {
		return nil
	}
	src := midir.LocalID(0)
	if len(in.Args) > 0 {
		src = in.Args[0]
	}
	switch {
	case in.Op == midir.OpBorrow && rf.track[src] && f.Locals[src].Ent != 0:
		s.ptr.set(in.Dst, rf.read(s, src))
	case in.Op == midir.OpBorrow:
		s.ptr.set(in.Dst, rf.lent)
	case in.Op == midir.OpAlias && rf.track[src] && rf.counted[in.Dst] && rf.crossedFromTemp(p, bid, i, src):
		s.release(src)
		s.fresh(in.Dst, rf.site[rf.siteBase[bid]+i], rf.loop[bid])
	case in.Op == midir.OpAlias && rf.track[src]:
		s.ptr.set(in.Dst, rf.read(s, src))
	case in.Op == midir.OpMove && rf.track[src]:
		s.ptr.set(in.Dst, rf.read(s, src))
		if f.Locals[src].Ent != 0 && src != in.Dst {
			s.ptr.set(src, rf.nulled)
		}
	case (in.Op == midir.OpBoxDirect || in.Op == midir.OpUnboxDirect) && f.Locals[in.Dst].Cross && rf.track[src]:
		s.ptr.set(in.Dst, s.ptr.get(src))
	case rf.counted[in.Dst]:
		s.fresh(in.Dst, rf.site[rf.siteBase[bid]+i], rf.loop[bid])
	default:
		s.ptr.set(in.Dst, rf.lent)
	}
	return nil
}

// read is what a value copied out of the local holds: for a binding that is only
// assigned on some paths, the read is on the assigning one, so only live objects.
func (rf *refFlow) read(s *refState, l midir.LocalID) int32 {
	if rf.weak != nil && rf.weak[l] {
		return s.sets.held(s.ptr.get(l))
	}
	return s.ptr.get(l)
}

func (p *verifier) takesForGood(f *midir.Func, in *midir.Inst, k int) bool {
	switch in.Op {
	case midir.OpCall, midir.OpCallValue:
		return ownquery.ConsumesArg(p.R, p.ByEnt, in, k) && !p.lentParam(*in, k) && !p.lentByType(f, in, k)
	case midir.OpRecord:
		return !p.borrowField(in.Ent, k)
	case midir.OpVariant, midir.OpArray:
		return true
	case midir.OpSetField:
		return k == 1 && !p.lentStore(f, *in, k)
	case midir.OpCellSet:
		return k == 1
	case midir.OpSetIndex:
		return k == 2
	case midir.OpNewCell:
		return k == 0 && !in.Lent
	case midir.OpGlobalSet, midir.OpBox, midir.OpAwait:
		return k == 0
	case midir.OpBoxDirect, midir.OpUnboxDirect:
		return !f.Locals[in.Dst].Cross
	}
	return false
}

func (p *Program) lentByType(f *midir.Func, in *midir.Inst, k int) bool {
	if in.Op != midir.OpCallValue || in.Ent != 0 || k == 0 {
		return false
	}
	t := p.fnType(p.derefAll(f.Locals[in.Args[0]].Type))
	if shape := p.R.FnShapeOf(t); shape != 0 {
		t = shape
	}
	node := p.R.Types.Node(t)
	return node.Kind == sem.KFn && k-1 < len(node.Args) && p.R.Types.Kind(node.Args[k-1]) == sem.KRef
}
