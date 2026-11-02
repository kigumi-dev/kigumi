package reach

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
