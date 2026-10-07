package mir

func (p *Program) directUses(f *Func, cand []bool) []bool {
	out := make([]bool, len(f.Locals))
	for i, prm := range f.Params {
		if f.DSig.Param(i) != 0 {
			out[prm] = true
		}
	}
	nullary := nullaryLocals(f)
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			p.markDirectUse(in, out)
			p.markPrimResult(f, in, out)
			markInlineArgs(in, out, cand, nullary)
		}
		if blk.Term.Op == TermReturn && f.DSig.result() != 0 {
			out[blk.Term.Args[0]] = true
		}
	}
	spreadDirectUse(f, out, cand)
	return out
}

// Building a payload-free variant or matching a carrier keeps nothing unboxed: unboxing to match would box the payload again.
func (p *Program) markDirectUse(in Inst, out []bool) {
	switch in.Op {
	case OpRecord:
		out[in.Dst] = true
	case OpVariant:
		out[in.Dst] = len(in.Args) > 0 || out[in.Dst]
	case OpField, OpFieldMove, OpSetField:
		out[in.Args[0]] = true
	case OpCall:
		fn, ok := p.ByEnt[in.Ent]
		if !ok || fn.DSig == nil {
			return
		}
		for i, a := range in.Args {
			if fn.DSig.Param(i) != 0 {
				out[a] = true
			}
		}
		if fn.DSig.Ret != 0 {
			out[in.Dst] = true
		}
	}
}

func spreadDirectUse(f *Func, out, cand []bool) {
	for changed := true; changed; {
		changed = false
		join := func(a, b LocalID) {
			if out[a] != out[b] {
				out[a], out[b], changed = true, true, true
			}
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				switch in.Op {
				case OpField, OpFieldMove, OpPayload:
					if cand[in.Dst] {
						join(in.Dst, in.Args[0])
					}
				case OpMove, OpCopy, OpShare, OpAlias:
					if cand[in.Dst] || cand[in.Args[0]] {
						join(in.Dst, in.Args[0])
					}
				}
			}
		}
	}
}

func markInlineArgs(in Inst, out, cand, nullary []bool) {
	args := in.Args
	switch {
	case (in.Op == OpRecord || in.Op == OpVariant) && cand[in.Dst]:
	case in.Op == OpSetField && cand[in.Args[0]]:
		args = in.Args[1:]
	default:
		return
	}
	for _, a := range args {
		if nullary[a] && cand[a] {
			out[a] = true
		}
	}
}

func nullaryLocals(f *Func) []bool {
	out := make([]bool, len(f.Locals))
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpVariant && len(in.Args) == 0 {
				out[in.Dst] = true
			}
		}
	}
	return out
}
