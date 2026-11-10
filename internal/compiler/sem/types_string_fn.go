package sem

func (r *Result) writeFn(w *typeWriter, n typeNode) { r.writeFnWith(w, n, r.writeType) }

func (r *Result) writeFnWith(w *typeWriter, n typeNode, write func(*typeWriter, TypeID)) {
	if n.Flags&fnEscaping != 0 {
		w.write("escaping ")
	}
	if n.Flags&fnCAbi != 0 {
		w.write("extern(C) ")
	}
	eff := Effects(n.Flags)
	if n.Flags&fnEffectPoly != 0 {
		w.write("pure? ")
		eff &^= EffPure
	}
	for _, e := range []struct {
		bit  Effects
		name string
	}{{EffPure, "pure "}, {EffNoalloc, "noalloc "}, {EffAsync, "async "}, {EffUnsafe, "unsafe "}} {
		if eff&e.bit != 0 {
			w.write(e.name)
		}
	}
	w.write("fn(")
	for i, a := range n.Args {
		if i > 0 {
			w.write(", ")
		}
		if r.Types.escMarked(n.Var, i) {
			w.write("escaping ")
		}
		if i == len(n.Args)-1 && n.Flags&fnVariadic != 0 {
			w.write("...")
			if elem, ok := r.arrayElem(a); ok {
				a = elem
			}
		}
		write(w, a)
	}
	if n.Flags&fnCVariadic != 0 {
		if len(n.Args) > 0 {
			w.write(", ")
		}
		w.write("...")
	}
	w.write(") -> ")
	write(w, n.Elem)
}

func (r *Result) arrayElem(t TypeID) (TypeID, bool) {
	n := r.Types.nodes[t]
	if n.Kind == KNamed && n.Ent == r.Types.arrayEnt && r.Types.arrayEnt != 0 && len(n.Args) == 1 {
		return n.Args[0], true
	}
	return 0, false
}
