package reach

import "kigumi/internal/compiler/sem"

// witnessBody is what the method-table slot of wit runs for a box of type t: the body specialize made for
// that instantiation, or the generic one when none needs it.
func (w *reachWalk) witnessBody(wit sem.EntityID, t sem.TypeID) sem.EntityID {
	if !w.desc.Specialized(wit) {
		return wit
	}
	if f := w.desc.Get(w.r, wit, t); f != nil {
		return f.Ent
	}
	return 0
}

// rootInstanceDrop keeps the drop body a value of the instantiation t of ent runs.
func (w *reachWalk) rootInstanceDrop(t sem.TypeID, ent sem.EntityID) {
	if w.r.Entity(ent).Kind != sem.EntType {
		return
	}
	drop := w.r.TypeDecl(ent).Drop
	if f := w.desc.Get(w.r, drop, t); f != nil {
		w.root(f.Ent, RootDropGlue)
	}
}
