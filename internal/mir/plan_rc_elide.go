package mir

import "kigumi/internal/sem"

// ElideRC drops counts PlanRC left that no observer can tell from their absence.
func ElideRC(p *Program) error {
	for _, f := range p.allFuncs() {
		for b := range f.Blocks {
			f.Blocks[b].Insts = p.elideBlock(f, f.Blocks[b].Insts)
		}
	}
	return nil
}

// Unit and Bool are the runtime's immortal singletons, so a count on one changes nothing.
func immortalLocal(f *Func, l LocalID) bool {
	t := f.Locals[l].Type
	return t == sem.TyUnit || t == sem.TyBool
}

func (p *Program) elideBlock(f *Func, in []Inst) []Inst {
	out := make([]Inst, 0, len(in))
	for k := 0; k < len(in); k++ {
		x := in[k]
		switch {
		case x.Op == OpRelease && immortalLocal(f, x.Args[0]):
			continue
		case x.Op == OpShare && immortalLocal(f, x.Args[0]):
			x = Inst{Op: OpAlias, Dst: x.Dst, Args: x.Args, Type: x.Type, Node: x.Node}
		case k+1 < len(in):
			if keep, skip := p.elidePair(f, x, in[k+1]); skip {
				k++
				if keep != nil {
					out = append(out, *keep)
				}
				continue
			}
		}
		out = append(out, x)
	}
	return out
}

// elidePair cancels a retain the next instruction releases, or hands the source's count to the retained temporary.
func (p *Program) elidePair(f *Func, share, next Inst) (keep *Inst, skip bool) {
	if share.Op != OpShare || share.Copy || next.Op != OpRelease || len(share.Args) != 1 {
		return nil, false
	}
	src, dst, rel := share.Args[0], share.Dst, next.Args[0]
	if f.IsDirect(src) || f.IsDirect(dst) || src == dst {
		return nil, false
	}
	switch {
	case rel == dst && f.Locals[dst].Ent == 0 && !f.Locals[dst].Cell:
		return nil, true
	case rel == src && f.Locals[src].Ent == 0 && !f.Locals[src].Cell:
		return &Inst{Op: OpMove, Dst: dst, Args: []LocalID{src}, Type: share.Type, Node: share.Node}, true
	}
	return nil, false
}
