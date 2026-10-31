package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// ReferentParams marks the parameters of an async body its Future takes as the referent itself when a call through its
// value lends a `&mut` write-back cell: a whole `&mut T` the body never replaces (sem rejects the replacing kind).
func (p *Program) ReferentParams(f *midir.Func) []bool {
	out := make([]bool, len(f.Params))
	for i, prm := range f.Params {
		l := f.Locals[prm]
		_, mut := p.R.MutRefElem(l.Type)
		_, scalar := p.R.ScalarRefElem(l.Type)
		out[i] = mut && !scalar && l.Ent != 0 && !p.R.RefCellParam(l.Ent)
	}
	return out
}
