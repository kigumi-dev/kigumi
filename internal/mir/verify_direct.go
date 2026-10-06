package mir

// Every crossing between the unboxed and boxed forms must go through OpBoxDirect/OpUnboxDirect.
func (p *Program) verifyDirect(f *Func) *Error {
	if !anyDirect(f) {
		return nil
	}
	for bid, blk := range f.Blocks {
		bid := BlockID(bid)
		for i, in := range blk.Insts {
			if err := p.verifyDirectInst(f, bid, i, in); err != nil {
				return err
			}
		}
		if blk.Term.Op == TermReturn && f.DSig.result() == 0 && f.IsDirect(blk.Term.Args[0]) {
			return fail("direct", bid, len(blk.Insts), "returns the unboxed %s without a box", p.local(f, blk.Term.Args[0]))
		}
	}
	return nil
}

func (p *Program) verifyDirectInst(f *Func, bid BlockID, i int, in Inst) *Error {
	if in.Op == OpBoxDirect {
		if !f.IsDirect(in.Args[0]) || f.IsDirect(in.Dst) {
			return fail("direct", bid, i, "boxdirect must take an unboxed local and yield a carrier")
		}
		return nil
	}
	if in.Op == OpUnboxDirect {
		if f.IsDirect(in.Args[0]) || !f.IsDirect(in.Dst) {
			return fail("direct", bid, i, "unboxdirect must take a carrier and yield an unboxed local")
		}
		return nil
	}
	for j, a := range in.Args {
		if f.IsDirect(a) && !p.directArg(f, in, j) {
			return fail("direct", bid, i, "%s reads the unboxed %s without a box", in.Op, p.local(f, a))
		}
	}
	if in.Op.Yields() && f.IsDirect(in.Dst) && !p.directDst(f, in) {
		return fail("direct", bid, i, "%s writes the carrier it yields into the unboxed %s", in.Op, p.local(f, in.Dst))
	}
	return nil
}
