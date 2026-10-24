package mir

import "kigumi/internal/sem"

func (rf *refFlow) block(p *Program, bid BlockID, entry *refState, check bool) (*refState, *Error) {
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
		if in.Op != OpDrop && in.Op != OpRelease {
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

func (rf *refFlow) step(p *Program, s *refState, bid BlockID, i int, in *Inst, broken func(int, string, ...any) *Error) *Error {
	f := rf.f
	switch in.Op {
	case OpDrop, OpRelease:
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
		if in.Op == OpDrop && named {
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
	src := LocalID(0)
	if len(in.Args) > 0 {
		src = in.Args[0]
	}
	switch {
	case in.Op == OpBorrow && rf.track[src] && f.Locals[src].Ent != 0:
		s.ptr.set(in.Dst, rf.read(s, src))
	case in.Op == OpBorrow:
		s.ptr.set(in.Dst, rf.lent)
	case in.Op == OpAlias && rf.track[src] && rf.counted[in.Dst] && rf.crossedFromTemp(p, bid, i, src):
		s.release(src)
		s.fresh(in.Dst, rf.site[rf.siteBase[bid]+i], rf.loop[bid])
	case in.Op == OpAlias && rf.track[src]:
		s.ptr.set(in.Dst, rf.read(s, src))
	case in.Op == OpMove && rf.track[src]:
		s.ptr.set(in.Dst, rf.read(s, src))
		if f.Locals[src].Ent != 0 && src != in.Dst {
			s.ptr.set(src, rf.nulled)
		}
	case (in.Op == OpBoxDirect || in.Op == OpUnboxDirect) && f.Locals[in.Dst].Cross && rf.track[src]:
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
func (rf *refFlow) read(s *refState, l LocalID) int32 {
	if rf.f.weakDefs != nil && rf.f.weakDefs[l] {
		return s.sets.held(s.ptr.get(l))
	}
	return s.ptr.get(l)
}

func (p *Program) takesForGood(f *Func, in *Inst, k int) bool {
	switch in.Op {
	case OpCall, OpCallValue:
		return p.consumesArg(in, k) && !p.lentParam(*in, k) && !p.lentByType(f, in, k)
	case OpRecord:
		return !p.borrowField(in.Ent, k)
	case OpVariant, OpArray:
		return true
	case OpSetField, OpCellSet:
		return k == 1
	case OpSetIndex:
		return k == 2
	case OpNewCell:
		return k == 0 && !in.Lent
	case OpGlobalSet, OpBox, OpAwait:
		return k == 0
	case OpBoxDirect, OpUnboxDirect:
		return !f.Locals[in.Dst].Cross
	}
	return false
}

func (p *Program) lentByType(f *Func, in *Inst, k int) bool {
	if in.Op != OpCallValue || in.Ent != 0 || k == 0 {
		return false
	}
	t := p.fnType(p.derefAll(f.Locals[in.Args[0]].Type))
	if shape := p.R.FnShapeOf(t); shape != 0 {
		t = shape
	}
	node := p.R.Types.Node(t)
	return node.Kind == sem.KFn && k-1 < len(node.Args) && p.R.Types.Kind(node.Args[k-1]) == sem.KRef
}
