package sem

import "kigumi/internal/syntax"

func (r *Result) newInitThunk(id EntityID, node syntax.NodeID) EntityID {
	e := r.Entities[id]
	ts := fnDecl(r.tree(e.File), node)
	th := Entity{Kind: EntFn, Name: e.Name, Pkg: e.Pkg, File: e.File, Node: node, Tok: ts.Name, Vis: Visibility{Level: VisPrivate, Scope: e.Pkg}, Flags: e.Flags & EfStd}
	tid := r.newEntity(th)
	r.Entities[tid].Detail = r.addFn(FnInfo{Body: ts.Body, InitOf: id})
	r.Files[e.File].Defs[node] = tid
	r.declareFn(tid)
	return tid
}

func (r *Result) declareComptimeConst(id EntityID, s constSlots) {
	e := &r.Entities[id]
	e.Type = r.resolveType(e.File, r.fileScopes[e.File], s.Type, posConst)
	info := r.constInfo(id)
	info.Thunk = r.newInitThunk(id, s.Thunk)
	info.State = constDone
}

func (r *Result) ConstThunk(id EntityID) EntityID {
	if r.Entities[id].Kind != EntConst {
		return 0
	}
	return r.constInfo(id).Thunk
}

func (r *Result) comptimeConstUse(f FileID, n syntax.NodeID, ent EntityID) bool {
	if r.Entities[ent].Kind != EntConst || r.evalConstDecl(ent).Kind != constNone || r.constInfo(ent).Thunk == 0 {
		return false
	}
	r.errAt(f, n, cComptimeConstUse, r.Entities[ent].Name)
	return true
}

func (r *Result) StaticPending(id EntityID) bool {
	info := r.Static(id)
	if info.Thunk == 0 {
		return false
	}
	_, ok := r.Files[r.Entities[id].File].Literals[info.Init]
	return !ok
}
