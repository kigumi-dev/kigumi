package mir

import "slices"

type variantFlow struct {
	p       *Program
	f       *Func
	cond    []bool
	lenOf   map[LocalID]LocalID
	carried []bool
}

func (p *Program) verifyVariants(f *Func) *Error {
	if !p.needsFacts(f) {
		return nil
	}
	cond := condLocals(f)
	vf := &variantFlow{p: p, f: f, cond: cond, lenOf: iterLengths(f), carried: carriedBools(f, cond)}
	in := make([]*variantFacts, len(f.Blocks))
	in[0] = newFacts(len(f.Locals))
	var found *Error
	acyclic := solveForward(f, func(bid BlockID, wake func(BlockID), final bool) bool {
		out, err := vf.block(bid, in[bid], final)
		if err != nil {
			found = err
			return false
		}
		if diverges(f, &f.Blocks[bid]) {
			return true
		}
		term := f.Blocks[bid].Term
		for k, succ := range term.Targets {
			edge := out
			if k < len(term.Targets)-1 {
				edge = out.clone()
			}
			if term.Op == TermBranch && !edge.assume(term.Args[0], k == 0) {
				continue
			}
			edge.keepOnly(vf.carried)
			edge.fold()
			switch {
			case in[succ] == nil:
				in[succ] = edge
				wake(succ)
			case in[succ].join(edge):
				wake(succ)
			}
		}
		return true
	})
	if found != nil || acyclic {
		return found
	}
	for bid := range f.Blocks {
		if in[bid] == nil {
			continue
		}
		if _, err := vf.block(BlockID(bid), in[bid], true); err != nil {
			return err
		}
	}
	return nil
}

func (p *Program) needsFacts(f *Func) bool {
	if slices.Contains(f.weakDefs, true) {
		return true
	}
	if cellReadAfterTake(f) {
		return true
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpBuiltin && iterAt(in.Str) {
				return true
			}
			if in.Op != OpPayload {
				continue
			}
			kind, t := p.aggOf(f.Locals[in.Args[0]].Type)
			if kind != aggAdt {
				continue
			}
			for _, v := range p.R.AggVariants(t) {
				if in.Index >= len(p.R.Variant(v).Payload) {
					return true
				}
			}
		}
	}
	return false
}

func condLocals(f *Func) []bool {
	cond := make([]bool, len(f.Locals))
	for _, blk := range f.Blocks {
		if blk.Term.Op == TermBranch {
			cond[blk.Term.Args[0]] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op.Yields() && cond[in.Dst] && copiesBool(in) && !cond[in.Args[0]] {
					cond[in.Args[0]], changed = true, true
				}
			}
		}
	}
	return cond
}

func copiesBool(in Inst) bool {
	switch in.Op {
	case OpAlias, OpCopy, OpMove, OpShare:
		return len(in.Args) == 1 && in.Args[0] != in.Dst
	case OpUnary:
		return in.Str == "!" && in.Args[0] != in.Dst
	}
	return false
}
