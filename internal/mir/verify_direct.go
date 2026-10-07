package mir

// A position and the local reaching it always agree on the form, so every
// crossing is an OpBoxDirect/OpUnboxDirect and no backend picks for itself.
func (p *Program) verifyDirect(f *Func) *Error {
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

func (p *Program) verifyDirectReturn(f *Func, bid BlockID, blk Block) *Error {
	a := blk.Term.Args
	if blk.Term.Op != TermReturn {
		return nil
	}
	if f.IsDirect(a[0]) != (f.DSig.result() != 0) {
		return fail("direct", bid, len(blk.Insts), "returns %s in the form the signature does not carry", p.local(f, a[0]))
	}
	return nil
}

func (p *Program) verifyDirectInst(f *Func, bid BlockID, i int, in Inst) *Error {
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
	if msg := p.directShape(f, in); msg != "" {
		return fail("direct", bid, i, "%s %s", in.Op, msg)
	}
	return nil
}
