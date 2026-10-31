package mir

// soleIndex answers soleDef for a whole function in one pass; valid only while the function is not rewritten.
type soleIndex struct {
	f     *Func
	defs  []*Inst
	multi []bool
}

func newSoleIndex(f *Func) *soleIndex {
	ix := &soleIndex{f: f, defs: make([]*Inst, len(f.Locals)), multi: make([]bool, len(f.Locals))}
	for _, blk := range f.Blocks {
		for k := range blk.Insts {
			if d := &blk.Insts[k]; d.Op.Yields() {
				if ix.defs[d.Dst] != nil {
					ix.multi[d.Dst] = true
				}
				ix.defs[d.Dst] = d
			}
		}
	}
	return ix
}

func (p *callVerifier) soleDefOf(f *Func, l LocalID) *Inst {
	if ix := p.sole; ix != nil && ix.f == f {
		if ix.multi[l] {
			return nil
		}
		return ix.defs[l]
	}
	return soleDef(f, l)
}
