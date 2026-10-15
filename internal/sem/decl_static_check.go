package sem

// declareStatics resolves each static's type and compile-time initializer, right after declareConsts.
func (r *Result) declareStatics() {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind == EntStatic && e.File != 0 && e.Flags&EfPoison == 0 {
			r.declareStatic(EntityID(id))
		}
	}
}

func (r *Result) declareStatic(id EntityID) {
	e := &r.Entities[id]
	info := r.Static(id)
	t := r.tree(e.File)
	s := staticDecl(t, e.Node)
	scope := r.fileScopes[e.File]
	typ := r.resolveType(e.File, scope, s.Type, posConst)
	e.Type = typ

	switch info.Owner {
	case StaticExtern:
		if s.Value != 0 {
			r.errAt(e.File, s.Value, cStaticExternInit)
		}
		if typ != TyPoison && !r.abiSafe(typ) {
			r.errAt(e.File, s.Type, r.abiCode(typ, cStaticExternAbiType), typ)
		}
		return
	case StaticExport:
		if typ != TyPoison && !r.abiSafe(typ) {
			r.errAt(e.File, s.Type, r.abiCode(typ, cStaticExternAbiType), typ)
		}
	}

	if s.Value == 0 {
		r.errAt(e.File, e.Node, cStaticNoInit, e.Name)
		return
	}
	if typ != TyPoison && !r.staticInitializable(typ) {
		r.errAt(e.File, s.Type, cStaticNotInitable, typ)
	}
	v, ok := r.evalConst(e.File, s.Value)
	if !ok {
		return
	}
	if typ == 0 {
		typ = r.untypedOf(v)
		e.Type = typ
	}
	r.constFits(e.File, s.Value, v, typ)
	info.Value = v
	info.State = constDone
}

// FixedArray is rejected too: no compile-time constant builds one.
func (r *Result) staticInitializable(t TypeID) bool {
	if !r.IsCopy(t) {
		return false
	}
	if n := r.Types.Node(t); n.Kind == KNamed && n.Ent == r.langItem(0, 0, "FixedArray") {
		return false
	}
	return true
}
