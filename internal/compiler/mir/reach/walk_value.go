package reach

import "kigumi/internal/compiler/sem"

// Only a record literal names a descriptor; the back end pools scalars and strings by kind.
func (w *reachWalk) walkValue(lit sem.Literal, t sem.TypeID) {
	n := w.r.Types.Node(t)
	if lit.Kind != sem.LitRecord || n.Kind != sem.KNamed {
		return
	}
	w.markType(n.Ent)
	for i, f := range w.r.TypeDecl(n.Ent).Fields {
		if i < len(lit.Fields) {
			w.walkValue(lit.Fields[i], w.r.Entity(f).Type)
		}
	}
}
