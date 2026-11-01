package plan

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

// storesPlaceCopy: a slot or field store into storage the function does not own takes a copy the store merges into the destination box.
func (w *shareFacts) storesPlaceCopy(in midir.Inst, i int) bool {
	v, ok := mir.StoredValue(in)
	if !ok || in.Args[i] != v || len(w.defs[v]) != 1 || len(w.uses[v]) != 1 {
		return false
	}
	if op := w.defs[v][0].Op; op != midir.OpIndex && op != midir.OpField {
		return false
	}
	if w.foreign == nil {
		w.foreign = foreignLocals(w.f)
	}
	return w.foreign[in.Args[0]] && w.p.R.InPlaceCopy(w.f.Locals[v].Type)
}
