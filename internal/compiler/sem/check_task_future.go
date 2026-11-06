package sem

import "kigumi/internal/compiler/syntax"

const (
	futureAwaited = 1 + iota
	futureSpawned
)

// taskFutureCall is a call whose Future holds a `Task`; mut says it can also write one through `&mut` or `mut self`.
type taskFutureCall struct {
	n   syntax.NodeID
	t   TypeID
	mut bool
}

func (r *Result) markTaskFuture(f FileID, n syntax.NodeID, how uint8) {
	if r.taskFutureUse == nil {
		r.taskFutureUse = map[taskEscapeKey]uint8{}
	}
	r.taskFutureUse[taskEscapeKey{f, r.tree(f).Unparen(n)}] = how
}

// noteSpawnArgs lets a Future handed straight to `task.spawn` or `Executor.handoff` keep its handles: the child stays in the scope.
func (r *Result) noteSpawnArgs(f FileID, n syntax.NodeID, call CallInfo) {
	if !r.isStdTaskEnt(call.Callee, "spawn", "handoff") {
		return
	}
	if call.Piped != 0 {
		r.markTaskFuture(f, call.Piped, futureSpawned)
	}
	if t := r.tree(f); t.Kind(n) == syntax.CallExpr || t.Kind(n) == syntax.WsCallExpr {
		for a := range t.View(n).AsCall().Args().IDs() {
			r.markTaskFuture(f, a, futureSpawned)
		}
	}
}

func (r *Result) taskFutureOf(f FileID, n syntax.NodeID, call CallInfo, ret TypeID) (taskFutureCall, bool) {
	direct := r.Types.Kind(ret) == KNamed && r.Types.Node(ret).Ent == r.Types.futureEnt
	if !direct && !r.containsFuture(ret, 0, map[EntityID]bool{}, map[TypeID]bool{}) {
		return taskFutureCall{}, false
	}
	var params []TypeID
	switch t := r.tree(f); {
	case call.Callee != 0 && r.Entities[call.Callee].Kind == EntFn:
		info := r.Fn(call.Callee)
		subst := map[EntityID]TypeID(nil)
		if len(info.TypeParams) == len(call.Inst) && len(call.Inst) > 0 {
			subst = instSubst(info.TypeParams, call.Inst)
		}
		for _, p := range append([]EntityID{info.SelfParam}, info.Params...) {
			if p != 0 {
				params = append(params, r.Types.Subst(r.Entities[p].Type, subst))
			}
		}
		if sp := info.SelfParam; sp != 0 && info.Recv == RecvMut {
			if st := params[0]; r.typeHasTask(st, map[TypeID]bool{}) {
				return taskFutureCall{n, st, true}, true
			}
		}
	case t.Kind(n) == syntax.CallExpr || t.Kind(n) == syntax.WsCallExpr:
		params = r.Types.Node(r.Files[f].Types[t.View(n).AsCall().Callee().ID()]).Args
	}
	var found taskFutureCall
	ok := false
	for _, p := range params {
		if r.mutRefHasTask(p, nil) {
			return taskFutureCall{n, p, true}, true
		}
		if !ok && r.typeHasTask(p, map[TypeID]bool{}) {
			found, ok = taskFutureCall{n, p, false}, true
		}
	}
	return found, ok
}

// checkTaskFutures rejects a Future holding a handle unless it is awaited at once, or, when it cannot write one back, spawned at once.
func (r *Result) checkTaskFutures(f FileID, calls []taskFutureCall) {
	for _, c := range calls {
		switch how := r.taskFutureUse[taskEscapeKey{f, c.n}]; {
		case how == futureAwaited, how == futureSpawned && !c.mut:
		case c.mut:
			r.errAt(f, c.n, cTaskHandleReturned, r.TypeString(c.t), "a write through a `&mut` or `mut self` parameter of a Future that is not awaited where it is built")
		default:
			r.errAt(f, c.n, cTaskHandleReturned, r.TypeString(c.t), "an argument of a Future that is not awaited or spawned where it is built")
		}
	}
}

// containsFuture reports a value type that can carry a Future, including one
// inside a record, variant, array or interface. It looks through type
// arguments and declared fields without substituting, and is depth-capped so
// recursive aliases cannot loop.
func (r *Result) containsFuture(t TypeID, depth int, ents map[EntityID]bool, seen map[TypeID]bool) bool {
	if t == 0 || t == TyPoison || depth > 24 || seen[t] {
		return false
	}
	seen[t] = true
	n := r.Types.Node(t)
	if n.Kind == KNamed {
		e := r.Entities[n.Ent]
		if e.Kind == EntType && n.Ent == r.Types.futureEnt {
			return true
		}
		if e.Kind == EntType && !ents[n.Ent] {
			ents[n.Ent] = true
			decl := r.typeDecl(n.Ent)
			switch decl.Form {
			case FormDefined:
				if r.containsFuture(decl.Underlying, depth+1, ents, seen) {
					return true
				}
			case FormRecord, FormResource:
				for _, fld := range decl.Fields {
					if r.containsFuture(r.field(fld).Type, depth+1, ents, seen) {
						return true
					}
				}
			case FormAdt:
				for _, v := range decl.Variants {
					for _, pt := range r.variant(v).Payload {
						if r.containsFuture(pt, depth+1, ents, seen) {
							return true
						}
					}
				}
			}
			delete(ents, n.Ent)
		}
	}
	if n.Kind == KIface {
		return true
	}
	if r.containsFuture(n.Elem, depth+1, ents, seen) {
		return true
	}
	for _, a := range n.Args {
		if r.containsFuture(a, depth+1, ents, seen) {
			return true
		}
	}
	return false
}
