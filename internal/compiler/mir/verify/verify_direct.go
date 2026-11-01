package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

// A position and the local reaching it always agree on the form, so every
// crossing is an OpBoxDirect/OpUnboxDirect and no backend picks for itself.
func (p *verifier) verifyDirect(f *midir.Func) *Error {
	for bid, blk := range f.Blocks {
		bid := midir.BlockID(bid)
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

func (p *verifier) verifyDirectForms(f *midir.Func) *Error {
	for i, prm := range f.Params {
		if f.IsDirect(prm) != (f.DSig.Param(i) != 0) {
			return fail("direct", 0, 0, "parameter %s is in the form the signature does not carry", p.LocalName(f, prm))
		}
	}
	return nil
}

func (p *verifier) verifyDirectReturn(f *midir.Func, bid midir.BlockID, blk midir.Block) *Error {
	a := blk.Term.Args
	if blk.Term.Op != midir.TermReturn {
		return nil
	}
	if f.IsDirect(a[0]) != (f.DSig.Result() != 0) {
		return fail("direct", bid, len(blk.Insts), "returns %s in the form the signature does not carry", p.LocalName(f, a[0]))
	}
	return nil
}

func (p *verifier) verifyDirectInst(f *midir.Func, bid midir.BlockID, i int, in midir.Inst) *Error {
	if in.Op == midir.OpBoxDirect || in.Op == midir.OpUnboxDirect {
		boxing := in.Op == midir.OpBoxDirect
		if f.IsDirect(in.Args[0]) != boxing || f.IsDirect(in.Dst) == boxing {
			return fail("direct", bid, i, "%s must read the one form and yield the other", in.Op)
		}
		return nil
	}
	for j, a := range in.Args {
		if f.IsDirect(a) != (p.ArgPos(f, in, j) != 0) {
			return fail("direct", bid, i, "%s reads %s in the form this position does not take", in.Op, p.LocalName(f, a))
		}
	}
	if in.Op.Yields() && f.IsDirect(in.Dst) != (p.DstPos(f, in) != 0) {
		return fail("direct", bid, i, "%s writes %s in the form this position does not yield", in.Op, p.LocalName(f, in.Dst))
	}
	if in.Op == midir.OpCall && in.DPrim != opdef.DPNone && !p.PrimTypesFit(f, in, in.DPrim) {
		return fail("direct", bid, i, "call hands its direct primitive operands of other types")
	}
	if msg := p.directShape(f, in); msg != "" {
		return fail("direct", bid, i, "%s %s", in.Op, msg)
	}
	return nil
}

func (p *verifier) verifyFixedBoxes(f *midir.Func) *Error {
	var fixed []bool
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op != midir.OpBoxDirect || !in.Fixed {
				continue
			}
			if fixed == nil {
				fixed = p.FixedLocals(f)
			}
			if !fixed[in.Args[0]] {
				return fail("direct", midir.BlockID(bid), i, "boxes %s as fixed, but it is not a constant", p.LocalName(f, in.Args[0]))
			}
		}
	}
	return nil
}
