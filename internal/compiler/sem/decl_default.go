package sem

import "kigumi/internal/compiler/syntax"

func (r *Result) declareDefaults(id EntityID) {
	e := &r.Entities[id]
	f := e.File
	t := r.tree(f)
	s := fnDecl(t, e.Node)
	var defaulted []syntax.NodeID
	for p := range t.List(s.Params).IDs() {
		if t.View(p).AsParam().Default().Valid() {
			defaulted = append(defaulted, p)
		}
	}
	if len(defaulted) == 0 {
		return
	}
	if why := r.defaultForbidden(id, s); why != "" {
		r.errAt(f, defaulted[0], cDefaultNotAllowed, why)
		return
	}
	if !r.defaultsAreSuffix(f, s) {
		return
	}
	for _, p := range defaulted {
		if pid := r.Files[f].Defs[p]; pid != 0 {
			r.Entities[pid].Target = r.newDefaultThunk(id, p)
		}
	}
}

func (r *Result) defaultsAreSuffix(f FileID, s fnSlots) bool {
	t := r.tree(f)
	seen, ok := false, true
	for p := range t.List(s.Params).IDs() {
		v := t.View(p).AsParam()
		switch {
		case v.Flags()&syntax.FlagSelf != 0:
		case v.Default().Valid():
			seen = true
		case seen && v.Type().Valid():
			r.errAt(f, p, cDefaultOrder, v.Name().Text())
			ok = false
		}
	}
	return ok
}

func (r *Result) defaultForbidden(id EntityID, s fnSlots) string {
	info := r.Fn(id)
	switch {
	case info.Abi != "":
		return "an `extern(C)` function"
	case info.Export != "":
		return "an `export(C)` function"
	}
	t := r.tree(r.Entities[id].File)
	for p := range t.List(s.Params).IDs() {
		if t.View(p).AsParam().Flags()&syntax.FlagVariadic != 0 {
			return "a variadic function"
		}
	}
	if info.Recv == RecvMut && readsSelf(t, s) {
		return "a `mut self` method whose default reads `self`"
	}
	return ""
}

func readsSelf(t *syntax.Tree, s fnSlots) bool {
	var walk func(v syntax.View) bool
	walk = func(v syntax.View) bool {
		if v.Kind() == syntax.Ident && v.Tok().Text() == "self" {
			return true
		}
		for c := range v.Children() {
			if walk(c) {
				return true
			}
		}
		return false
	}
	for p := range t.List(s.Params).IDs() {
		if def := t.View(p).AsParam().Default(); def.Valid() && walk(def.View) {
			return true
		}
	}
	return false
}

// An interface requirement has no body to run a default for.
func (r *Result) rejectRequirementDefaults(f FileID, s fnSlots) {
	t := r.tree(f)
	for p := range t.List(s.Params).IDs() {
		if t.View(p).AsParam().Default().Valid() {
			r.errAt(f, p, cDefaultNotAllowed, "an interface requirement")
		}
	}
}

func (r *Result) newDefaultThunk(id EntityID, p syntax.NodeID) EntityID {
	e := r.Entities[id]
	t := r.tree(e.File)
	node := syntax.NodeID(t.View(p).AsParam().Thunk())
	ts := fnDecl(t, node)
	th := Entity{Kind: EntFn, Name: e.Name, Pkg: e.Pkg, File: e.File, Node: node, Tok: ts.Name, Vis: Visibility{Level: VisPrivate, Scope: e.Pkg}, Flags: e.Flags & EfStd}
	tid := r.newEntity(th)
	r.Entities[tid].Detail = r.addFn(FnInfo{Body: ts.Body, Declared: Effects(ts.Mods), DefaultOf: id, DefaultParam: t.View(p).Tok().Text(), DefaultTry: ts.Mods&syntax.ModDefaultTry != 0})
	r.Files[e.File].Defs[node] = tid
	r.declareFn(tid)
	return tid
}

func (r *Result) defaultFingerprint(param EntityID) string {
	e := &r.Entities[param]
	if e.Target == 0 {
		return ""
	}
	t := r.tree(e.File)
	return " default=" + tokenFingerprint(t, t.Span(t.View(e.Node).AsParam().Default().ID()))
}
