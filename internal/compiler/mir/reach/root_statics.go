package reach

import "kigumi/internal/compiler/sem"

// rootStatics keeps what kg_init_statics builds: it initializes every static at startup, whether or not code reads it.
func (w *reachWalk) rootStatics() {
	for _, ent := range w.r.StaticEntities() {
		typ := w.r.Entity(ent).Type
		w.walkType(typ)
		if lit, ok := w.r.StaticValueOf(ent); ok {
			w.walkValue(lit, typ)
		}
	}
}

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
