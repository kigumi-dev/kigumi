package mir

// verifyDropOwned rejects releasing a reference this frame took no count of:
// the referent would be freed under its real owner.
func (p *Program) verifyDropOwned(f *Func) *Error {
	lent := p.unownedRefs(f)
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op != OpDrop && in.Op != OpRelease {
				continue
			}
			if len(in.Args) == 1 && lent[in.Args[0]] {
				return fail("droplent", BlockID(bid), i, "releases %%%s, a reference this frame does not own", f.Locals[in.Args[0]].Name)
			}
		}
	}
	return nil
}
