package mir

import "kigumi/internal/compiler/sem"

func (p *Program) boxedType(f *Func, in Inst) sem.TypeID {
	switch in.Op {
	case OpBoxDirect:
		if in.Fixed {
			return 0
		}
		return in.Type
	case OpBorrow:
		return p.boxedLend(f, in)
	}
	return 0
}

func (p *Program) boxedLend(f *Func, in Inst) sem.TypeID {
	if in.Op != OpBorrow || in.Index != 0 || !p.R.LendSite(f.File, in.Node) {
		return 0
	}
	if t := f.Locals[in.Args[0]].Type; p.R.DirectAgg(t) && !p.nullaryDef(f, in.Args[0]) {
		return t
	}
	return 0
}

// An undeclared body is noalloc only while its lends stay direct; one that boxes allocates after all, which callers learn here.
// A witness bind that survives planning allocates, declared `noalloc` or not.
func (p *Program) demoteBoxedLends(reached map[sem.EntityID]bool) {
	for _, f := range p.allFuncs() {
		if f.Ent == 0 || p.sharedUnreached(f, reached) {
			continue
		}
		declared := p.declaredNoalloc(f)
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !declared && p.boxedLend(f, in) != 0 || p.keptWitnessBind(f, in) {
					p.R.DemoteBoxedLend(f.File, in.Node)
				}
			}
		}
	}
	p.R.SettleLendDemotions()
}

func (p *Program) keptWitnessBind(f *Func, in Inst) bool {
	return in.Op == OpBind && len(in.Args) > 0 && p.R.LendSite(f.File, in.Node)
}

// A payload-free variant is one immortal every site shares, so borrowing it costs nothing.
func (p *Program) nullaryDef(f *Func, l LocalID) bool {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() && in.Dst == l {
				return in.Op == OpVariant && len(in.Args) == 0
			}
		}
	}
	return false
}
