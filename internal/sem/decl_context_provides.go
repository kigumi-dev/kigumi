package sem

import "kigumi/internal/syntax"

// ProvideBinding is one context key a provider type's field supplies via `@provides(key, ...)`.
type ProvideBinding struct {
	Key   EntityID
	Field EntityID
}

func (r *Result) checkContextProviders() {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind != EntType || e.File == 0 || e.Flags&EfPoison != 0 {
			continue
		}
		s := typeDecl(r.tree(e.File), e.Node)
		if s.Attrs != 0 {
			r.providesAttr(EntityID(id), s.Attrs)
		}
	}
}

func (r *Result) providesAttr(id EntityID, attrs syntax.NodeID) {
	f := r.Entities[id].File
	t := r.tree(f)
	scope := r.declScope(id)
	for _, a := range t.Children(attrs) {
		if t.Kind(a) != syntax.Attr {
			continue
		}
		if pathText(t, syntax.NodeID(t.Nodes[a].Lhs), ".") != "provides" {
			continue
		}
		argsList := syntax.NodeID(t.Nodes[a].Rhs)
		if argsList == 0 {
			continue
		}
		seen := map[EntityID]bool{}
		for _, arg := range t.Children(argsList) {
			r.providesArg(id, f, scope, arg, seen)
		}
	}
}

func (r *Result) providesArg(id EntityID, f FileID, scope ScopeID, arg syntax.NodeID, seen map[EntityID]bool) {
	t := r.tree(f)
	if t.Kind(arg) == syntax.Ident {
		name := t.TokText(t.Nodes[arg].Tok)
		if b, _, ok := r.lookup(scope, name); ok && b.Ent == 0 && b.Set != 0 {
			r.errAt(f, arg, cCtxProvidesNotKey, name)
			return
		}
	}
	ent, _, ok := r.exprAsTypeRef(f, scope, arg)
	if !ok {
		return
	}
	if r.Entities[ent].Kind != EntContextKey {
		r.errAt(f, arg, cCtxProvidesNotKey, r.Entities[ent].Name)
		return
	}
	if seen[ent] {
		r.errAt(f, arg, cCtxProvidesDuplicate, ent)
		return
	}
	seen[ent] = true
	name := r.Entities[ent].Name
	selfType := r.Types.Named(id, nil)
	fld := r.findField(id, name)
	if fld == 0 {
		r.errAt(f, arg, cCtxProvidesMissingField, selfType, ent)
		return
	}
	if r.field(fld).Type != r.Entities[ent].Type {
		r.errAt(f, arg, cCtxProvidesFieldType, ent, selfType, r.field(fld).Type, r.Entities[ent].Type)
		return
	}
	r.provides[id] = append(r.provides[id], ProvideBinding{Key: ent, Field: fld})
}

// providesOf returns the context keys a provider value's type supplies.
func (r *Result) providesOf(t TypeID) ([]ProvideBinding, bool) {
	n := r.Types.Node(t)
	if n.Kind != KNamed || n.Ent == 0 {
		return nil, false
	}
	list := r.provides[n.Ent]
	return list, len(list) > 0
}
