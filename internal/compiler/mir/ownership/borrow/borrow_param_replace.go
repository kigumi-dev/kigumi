package borrow

import (
	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// replacingUse reports whether in, given cell first, only replaces what it holds or reads it for a fieldStore.
func (p *Analyzer) replacingUse(f *midir.Func, in midir.Inst, cell midir.LocalID) bool {
	return in.Op == midir.OpCellSet || p.fieldStore(f, in, cell) || in.Op == midir.OpCellGet && p.onlyFieldStores(f, in.Dst, cell)
}

// fieldStore reports whether in replaces the only borrow-carrying field of what cell holds, directly or through a `cellget` of it.
func (p *Analyzer) fieldStore(f *midir.Func, in midir.Inst, cell midir.LocalID) bool {
	if in.Op != midir.OpSetField || len(in.Args) == 0 {
		return false
	}
	if t := in.Args[0]; t != cell {
		d := mirflow.SoleDef(f, t)
		if d == nil || d.Op != midir.OpCellGet || len(d.Args) == 0 || d.Args[0] != cell {
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
func (p *Analyzer) onlyFieldStores(f *midir.Func, v, cell midir.LocalID) bool {
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
