package mir

import (
	opdef "kigumi/internal/compiler/mir/ops"
)

// A position and the local reaching it always agree on the form, so every
// crossing is an OpBoxDirect/OpUnboxDirect and no backend picks for itself.
func (p *verifier) verifyDirect(f *Func) *Error {
	for bid, blk := range f.Blocks {
		bid := BlockID(bid)
		for i, in := range blk.Insts {
			if err := p.verifyDirectInst(f, bid, i, in); err != nil {
				return err
			}
		}
		if err := p.verifyDirectReturn(f, bid, blk); err != nil {
			return err
		}
	}
	return nil
}

func (p *verifier) verifyDirectForms(f *Func) *Error {
	for i, prm := range f.Params {
		if f.IsDirect(prm) != (f.DSig.Param(i) != 0) {
			return fail("direct", 0, 0, "parameter %s is in the form the signature does not carry", p.local(f, prm))
		}
	}
	return nil
}

func (p *verifier) verifyDirectReturn(f *Func, bid BlockID, blk Block) *Error {
	a := blk.Term.Args
	if blk.Term.Op != TermReturn {
		return nil
	}
	if f.IsDirect(a[0]) != (f.DSig.result() != 0) {
		return fail("direct", bid, len(blk.Insts), "returns %s in the form the signature does not carry", p.local(f, a[0]))
	}
	return nil
}

func (p *verifier) verifyDirectInst(f *Func, bid BlockID, i int, in Inst) *Error {
	if in.Op == OpBoxDirect || in.Op == OpUnboxDirect {
		boxing := in.Op == OpBoxDirect
		if f.IsDirect(in.Args[0]) != boxing || f.IsDirect(in.Dst) == boxing {
			return fail("direct", bid, i, "%s must read the one form and yield the other", in.Op)
		}
		return nil
	}
	for j, a := range in.Args {
		if f.IsDirect(a) != (p.argPos(f, in, j) != 0) {
			return fail("direct", bid, i, "%s reads %s in the form this position does not take", in.Op, p.local(f, a))
		}
	}
	if in.Op.Yields() && f.IsDirect(in.Dst) != (p.dstPos(f, in) != 0) {
		return fail("direct", bid, i, "%s writes %s in the form this position does not yield", in.Op, p.local(f, in.Dst))
	}
	if in.Op == OpCall && in.DPrim != opdef.DPNone && !p.primTypesFit(f, in, in.DPrim) {
		return fail("direct", bid, i, "call hands its direct primitive operands of other types")
	}
	if msg := p.directShape(f, in); msg != "" {
		return fail("direct", bid, i, "%s %s", in.Op, msg)
	}
	return nil
}

func (p *verifier) verifyFixedBoxes(f *Func) *Error {
	var fixed []bool
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op != OpBoxDirect || !in.Fixed {
				continue
			}
			if fixed == nil {
				fixed = p.fixedLocals(f)
			}
			if !fixed[in.Args[0]] {
				return fail("direct", BlockID(bid), i, "boxes %s as fixed, but it is not a constant", p.local(f, in.Args[0]))
			}
		}
	}
	return nil
}
