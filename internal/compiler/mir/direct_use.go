package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

func (p *Program) directUses(f *midir.Func, cand []bool) []bool {
	out := make([]bool, len(f.Locals))
	for i, prm := range f.Params {
		if f.DSig.Param(i) != 0 {
			out[prm] = true
		}
	}
	nullary := nullaryLocals(f)
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			p.markDirectUse(f, in, out)
			p.markPrimResult(f, in, out)
			markInlineArgs(in, out, cand, nullary)
		}
		if blk.Term.Op == midir.TermReturn && f.DSig.Result() != 0 {
			out[blk.Term.Args[0]] = true
		}
	}
	p.spreadDirectUse(f, out, cand)
	return out
}

// Building a payload-free variant or matching a carrier keeps nothing unboxed: unboxing to match would box the payload again.
func (p *Program) markDirectUse(f *midir.Func, in midir.Inst, out []bool) {
	p.fixedUse(f, in, out)
	switch in.Op {
	case midir.OpRecord, midir.OpClosure:
		out[in.Dst] = true
	case midir.OpCallValue:
		out[in.Args[0]] = true
		p.markValueCall(f, in, out)
	case midir.OpVariant:
		out[in.Dst] = len(in.Args) > 0 || out[in.Dst]
	case midir.OpField, midir.OpFieldMove, midir.OpSetField:
		out[in.Args[0]] = true
	case midir.OpCall:
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

func (p *Program) spreadDirectUse(f *midir.Func, out, cand []bool) {
	for changed := true; changed; {
		changed = false
		join := func(a, b midir.LocalID) {
			if out[a] != out[b] {
				out[a], out[b], changed = true, true, true
			}
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				switch in.Op {
				case midir.OpField, midir.OpFieldMove, midir.OpPayload:
					if cand[in.Dst] {
						join(in.Dst, in.Args[0])
					}
				case midir.OpIndex, midir.OpBuiltin:
					if (in.Op == midir.OpIndex || iterBuiltin(in)) && cand[in.Dst] && p.R.IsFixedArray(f.Locals[in.Args[0]].Type) {
						join(in.Dst, in.Args[0])
					}
				case midir.OpMove, midir.OpCopy, midir.OpShare, midir.OpAlias:
					if cand[in.Dst] || cand[in.Args[0]] {
						join(in.Dst, in.Args[0])
					}
				}
			}
		}
	}
}

func markInlineArgs(in midir.Inst, out, cand, nullary []bool) {
	args := in.Args
	switch {
	case (in.Op == midir.OpRecord || in.Op == midir.OpVariant || in.Op == midir.OpClosure) && cand[in.Dst]:
	case in.Op == midir.OpSetField && cand[in.Args[0]]:
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

func nullaryLocals(f *midir.Func) []bool {
	out := make([]bool, len(f.Locals))
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpVariant && len(in.Args) == 0 {
				out[in.Dst] = true
			}
		}
	}
	return out
}
