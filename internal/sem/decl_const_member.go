package sem

import "kigumi/internal/syntax"

func (r *Result) qualifiedConstPath(f FileID, n syntax.NodeID) (syntax.NodeID, bool) {
	t := r.tree(f)
	m := t.View(n).AsMember()
	base := m.Base()
	if base.Kind() != syntax.Ident {
		return 0, false
	}
	b, _, found := r.lookup(r.fileScopes[f], base.Tok().Text())
	if !found || b.Ent == 0 || r.Entities[b.Ent].Kind != EntImport {
		return 0, false
	}
	if p, cached := r.identPaths[t][n]; cached {
		return p, true
	}
	p := t.AddPath(base.Tok().Index(), []uint32{base.Tok().Index(), m.Member().Index()})
	if r.identPaths[t] == nil {
		r.identPaths[t] = map[syntax.NodeID]syntax.NodeID{}
	}
	r.identPaths[t][n] = p
	return p, true
}

func (r *Result) evalQualifiedConst(f FileID, n syntax.NodeID) (constValue, bool) {
	path, ok := r.qualifiedConstPath(f, n)
	if !ok {
		r.markImportsUsed(f, r.tree(f).View(n))
		r.errAt(f, n, cConstNotComptime, "this expression")
		return constValue{}, false
	}
	ent := r.resolveValueName(f, r.fileScopes[f], path)
	if ent == 0 {
		return constValue{}, false
	}
	r.Files[f].Uses[n] = ent
	if r.Entities[ent].Kind != EntConst {
		r.errAt(f, n, cConstNotComptime, "`"+r.Entities[ent].Name+"`")
		return constValue{}, false
	}
	if r.comptimeConstUse(f, n, ent) {
		return constValue{}, false
	}
	v := r.evalConstDecl(ent)
	return v, v.Kind != constNone
}
