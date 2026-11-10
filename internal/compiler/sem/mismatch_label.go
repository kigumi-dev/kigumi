package sem

// mismatchLabels qualifies want/got with their declaring package when they share a bare name, so the message never
// reads "expected `Widget`, found `Widget`".
func (r *Result) mismatchLabels(want, got TypeID) (string, string) {
	we, ge := r.namedTypeEntity(want), r.namedTypeEntity(got)
	if we != 0 && ge != 0 && we != ge {
		wf, gf := &r.Entities[we], &r.Entities[ge]
		if wf.Name == gf.Name && wf.Pkg != gf.Pkg {
			return r.qualifiedTypeString(want, we), r.qualifiedTypeString(got, ge)
		}
	}
	// Lifetime parameters of different declarations can both be spelled `'a`; qualify each with its declaring item.
	if qualify := r.collidingLifetimeParams(want, got); qualify != nil {
		ww, gw := newTypeWriter(), newTypeWriter()
		r.writeTypeQualifyingLifetimes(ww, want, qualify)
		r.writeTypeQualifyingLifetimes(gw, got, qualify)
		return ww.str(), gw.str()
	}
	return r.TypeString(want), r.TypeString(got)
}

func (r *Result) collidingLifetimeParams(a, b TypeID) map[EntityID]bool {
	names := map[string]EntityID{}
	for _, p := range r.Types.Params(a) {
		if r.typeParam(p).IsLifetime {
			names[r.Entities[p].Name] = p
		}
	}
	var out map[EntityID]bool
	for _, p := range r.Types.Params(b) {
		if !r.typeParam(p).IsLifetime {
			continue
		}
		if other, ok := names[r.Entities[p].Name]; ok && other != p {
			if out == nil {
				out = map[EntityID]bool{}
			}
			out[p], out[other] = true, true
		}
	}
	return out
}

func (r *Result) writeTypeQualifyingLifetimes(w *typeWriter, t TypeID, qualify map[EntityID]bool) {
	if w.spent() {
		w.truncated = true
		return
	}
	if w.depth >= typeRenderDepth {
		w.write(typeRenderElision)
		return
	}
	w.depth++
	defer func() { w.depth-- }()
	tt := r.Types
	n := tt.nodes[t]
	write := func(w *typeWriter, t TypeID) { r.writeTypeQualifyingLifetimes(w, t, qualify) }
	switch {
	case n.Kind == KParam && qualify[n.Ent]:
		w.write(r.entityName(n.Ent))
		w.write(" (")
		w.write(r.entityName(r.Entities[n.Ent].Parent))
		w.write(")")
		return
	case n.Kind == KFn:
		r.writeFnWith(w, n, write)
		return
	case n.Kind == KRef:
		w.write("&")
		if n.Lifetime != 0 && tt.nodes[n.Lifetime].Kind == KParam {
			write(w, n.Lifetime)
			w.write(" ")
		}
		if n.Flags&flagMut != 0 {
			w.write("mut ")
		}
		write(w, n.Elem)
		return
	}
	if n.Kind == KNamed || n.Kind == KIface {
		if _, ok := tt.IsOption(t); !ok {
			if _, _, ok := tt.IsResult(t); !ok && len(n.Args) > 0 {
				w.write(r.entityName(n.Ent))
				w.write("[")
				for i, a := range n.Args {
					if i > 0 {
						w.write(", ")
					}
					r.writeTypeQualifyingLifetimes(w, a, qualify)
				}
				w.write("]")
				return
			}
		}
	}
	r.writeType(w, t)
}

func (r *Result) namedTypeEntity(t TypeID) EntityID {
	if t == 0 {
		return 0
	}
	tt := r.Types
	n := tt.nodes[t]
	if n.Kind != KNamed && n.Kind != KIface {
		return 0
	}
	if _, ok := tt.IsOption(t); ok {
		return 0
	}
	if _, _, ok := tt.IsResult(t); ok {
		return 0
	}
	return n.Ent
}

func (r *Result) qualifiedTypeString(t TypeID, ent EntityID) string {
	w := newTypeWriter()
	if pkg := r.Entities[ent].Pkg; pkg != 0 && int(pkg) < len(r.Packages) {
		if path := r.Packages[pkg].Path; path != "" {
			w.write(path)
			w.write(".")
		}
	}
	w.write(r.entityName(ent))
	r.writeArgs(w, r.Types.nodes[t].Args)
	return w.str()
}
