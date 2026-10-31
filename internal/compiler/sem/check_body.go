package sem

func (r *Result) checkTestBody(id EntityID) {
	e := &r.Entities[id]
	body := r.tree(e.File).View(e.Node).AsTestDecl().Body().ID()
	scope := r.newScope(ScopeFn, r.fileScopes[e.File], e.File, e.Node)
	r.Scopes[scope].Fn = id
	var edges []EffectEdge
	c := r.newChecker(e.File, id, scope, r.unitResult(), &edges)
	r.Bodies = append(r.Bodies, BodyRef{Fn: id, File: e.File, Node: body})
	c.checkBodyBlock(body)
	c.settleExclusive()
	c.settleInlineAt(true)
	c.flushFieldWrites()
	c.dropsAtScopeExit(nil)
	r.Fn(id).Edges = edges
}

func (r *Result) unitResult() TypeID { return r.Types.Result(TyUnit, r.Types.errorType()) }
