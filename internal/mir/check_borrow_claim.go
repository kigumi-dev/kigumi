package mir

import "kigumi/internal/sem"

func (p *Program) boxedType(f *Func, in Inst) sem.TypeID {
	switch in.Op {
	case OpBoxDirect:
		return in.Type
	case OpBorrow:
		if t := f.Locals[in.Args[0]].Type; in.Index == 0 && p.R.LendSite(f.File, in.Node) && p.R.DirectAgg(t) && !p.nullaryDef(f, in.Args[0]) {
			return t
		}
	}
	return 0
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
