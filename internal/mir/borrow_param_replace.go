package mir

import "kigumi/internal/sem"

// replacingUse reports whether in, given cell first, only replaces what it holds or reads it for a fieldStore.
func (p *Program) replacingUse(f *Func, in Inst, cell LocalID) bool {
	return in.Op == OpCellSet || p.fieldStore(f, in, cell) || in.Op == OpCellGet && p.onlyFieldStores(f, in.Dst, cell)
}

// fieldStore reports whether in replaces the only borrow-carrying field of what cell holds, directly or through a `cellget` of it.
func (p *Program) fieldStore(f *Func, in Inst, cell LocalID) bool {
	if in.Op != OpSetField || len(in.Args) == 0 {
		return false
	}
	if t := in.Args[0]; t != cell {
		d := soleDef(f, t)
		if d == nil || d.Op != OpCellGet || len(d.Args) == 0 || d.Args[0] != cell {
			return false
		}
	}
	t := f.Locals[cell].Type
	for p.R.Types.Kind(t) == sem.KRef {
		t = p.R.Types.Node(t).Elem
	}
	return p.replacingField(t, in.Index)
}

// onlyFieldStores reports whether every use of the `cellget` result v is a fieldStore through it.
func (p *Program) onlyFieldStores(f *Func, v, cell LocalID) bool {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for k, a := range in.Args {
				if a == v && (k != 0 || !p.fieldStore(f, in, cell)) {
					return false
				}
			}
		}
		for _, a := range blk.Term.Args {
			if a == v {
				return false
			}
		}
	}
	return true
}
