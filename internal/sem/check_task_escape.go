package sem

import "kigumi/internal/syntax"

func (r *Result) isStdTaskEnt(ent EntityID, names ...string) bool {
	pkg, ok := r.pathIndex["std/task"]
	if !ok || ent == 0 || r.Entities[ent].Pkg != pkg {
		return false
	}
	for _, n := range names {
		if r.Entities[ent].Name == n {
			return true
		}
	}
	return false
}

// typeHasTask reports whether a value of t can hold a `Task` handle; a closure's captures are checked where it is built.
func (r *Result) typeHasTask(t TypeID, seen map[TypeID]bool) bool {
	if t == 0 || t == TyPoison || seen[t] {
		return false
	}
	seen[t] = true
	n := r.Types.Node(t)
	switch n.Kind {
	case KFn, KClosure:
		return false
	case KNamed:
		if r.Entities[n.Ent].Kind == EntType && r.isStdTaskEnt(n.Ent, "Task") {
			return true
		}
		if r.Entities[n.Ent].Kind == EntType && r.namedHasTask(n.Ent, n.Args, seen) {
			return true
		}
	}
	if r.typeHasTask(n.Elem, seen) {
		return true
	}
	for _, a := range n.Args {
		if r.typeHasTask(a, seen) {
			return true
		}
	}
	return false
}

func (r *Result) namedHasTask(ent EntityID, args []TypeID, seen map[TypeID]bool) bool {
	decl := r.typeDecl(ent)
	subst := instSubst(decl.Params, args)
	switch decl.Form {
	case FormDefined:
		return r.typeHasTask(r.Types.Subst(decl.Underlying, subst), seen)
	case FormRecord, FormResource:
		for _, fld := range decl.Fields {
			if r.typeHasTask(r.Types.Subst(r.field(fld).Type, subst), seen) {
				return true
			}
		}
	case FormAdt:
		for _, v := range decl.Variants {
			for _, pt := range r.variant(v).Payload {
				if r.typeHasTask(r.Types.Subst(pt, subst), seen) {
					return true
				}
			}
		}
	}
	return false
}

func (r *Result) checkAsyncTaskReturn(f FileID, at syntax.NodeID, sig TypeID) {
	if ret := r.Types.Node(sig).Elem; r.typeHasTask(ret, map[TypeID]bool{}) {
		r.errAt(f, at, cTaskHandleReturned, r.TypeString(ret), "the result of an async fn")
	}
}

func (r *Result) mutRefHasTask(t TypeID, subst map[EntityID]TypeID) bool {
	n := r.Types.Node(t)
	if n.Kind != KRef || n.Flags&flagMut == 0 {
		return false
	}
	elem := n.Elem
	if subst != nil {
		elem = r.Types.Subst(elem, subst)
	}
	return r.typeHasTask(elem, map[TypeID]bool{})
}

// checkTaskEscapeCall rejects a task tree whose result would carry a handle to one of its own children out of it.
func (r *Result) checkTaskEscapeCall(f FileID, n syntax.NodeID, call CallInfo) {
	if len(call.Inst) == 0 || !r.isStdTaskEnt(call.Callee, "spawn", "handoff", "runToCompletion") {
		return
	}
	if t := call.Inst[0]; r.typeHasTask(t, map[TypeID]bool{}) {
		r.errAt(f, n, cTaskHandleReturned, r.TypeString(t), "the result of a spawned or root task")
	}
}

type taskEscapeKey struct {
	f FileID
	n syntax.NodeID
}

func (r *Result) checkTaskEscapeArgs(f FileID, n syntax.NodeID, ent EntityID, args []TypeID) {
	if !r.isAllocType(ent, "Shared") {
		return
	}
	for _, a := range args {
		if r.typeHasTask(a, map[TypeID]bool{}) {
			key := taskEscapeKey{f, n}
			if r.taskEscapeSeen == nil {
				r.taskEscapeSeen = map[taskEscapeKey]bool{}
			}
			if !r.taskEscapeSeen[key] {
				r.taskEscapeSeen[key] = true
				r.errAt(f, n, cTaskHandleStored, r.TypeString(a), r.Entities[ent].Name)
			}
			return
		}
	}
}

// IsTaskType reports whether t is a `std/task` `Task[T]` handle.
func (r *Result) IsTaskType(t TypeID) bool {
	n := r.Types.Node(t)
	return n.Kind == KNamed && r.Entities[n.Ent].Kind == EntType && r.isStdTaskEnt(n.Ent, "Task")
}

// IsTaskYieldNow reports whether ent is `task.yieldNow`, whose Future holds no executor state.
func (r *Result) IsTaskYieldNow(ent EntityID) bool {
	return r.isStdTaskEnt(ent, "yieldNow")
}

// IsTaskRunToCompletion reports whether ent is `Executor.runToCompletion`, which finishes its future and every task it spawns before returning.
func (r *Result) IsTaskRunToCompletion(ent EntityID) bool {
	return r.Entities[ent].Kind == EntFn && r.isStdTaskEnt(ent, "runToCompletion")
}
