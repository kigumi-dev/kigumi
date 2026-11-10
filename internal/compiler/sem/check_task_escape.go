package sem

import "kigumi/internal/compiler/syntax"

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

func (r *Result) checkAsyncTaskReturn(f FileID, at syntax.NodeID, sig TypeID) {
	ret := r.Types.Node(sig).Elem
	r.rejectTaskHolder(f, at, ret, cTaskHandleReturned, r.TypeString(ret), "the result of an async fn")
}

func (r *Result) mutRefHolding(t TypeID, subst map[EntityID]TypeID) taskHolding {
	n := r.Types.Node(t)
	if n.Kind != KRef || n.Flags&flagMut == 0 {
		return taskHolding{}
	}
	elem := n.Elem
	if subst != nil {
		elem = r.Types.Subst(elem, subst)
	}
	return r.taskHoldingOf(elem)
}

// checkTaskEscapeCall rejects a task tree whose result would carry a handle to one of its own children out of it.
func (r *Result) checkTaskEscapeCall(f FileID, n syntax.NodeID, call CallInfo) {
	if len(call.Inst) == 0 || !r.isStdTaskEnt(call.Callee, "spawn", "handoff", "runToCompletion") {
		return
	}
	t := call.Inst[0]
	r.rejectTaskHolder(f, n, t, cTaskHandleReturned, r.TypeString(t), "the result of a spawned or root task")
}

type taskEscapeKey struct {
	f FileID
	n syntax.NodeID
}

func (r *Result) checkTaskEscapeArgs(f FileID, n syntax.NodeID, ent EntityID, args []TypeID) {
	if !r.isAllocType(ent, "Shared") {
		return
	}
	key := taskEscapeKey{f, n}
	if r.taskEscapeSeen[key] {
		return
	}
	for _, a := range args {
		if !r.taskHoldingOf(a).any() {
			continue
		}
		if r.rejectTaskHolder(f, n, a, cTaskHandleStored, r.TypeString(a), r.Entities[ent].Name) {
			if r.taskEscapeSeen == nil {
				r.taskEscapeSeen = map[taskEscapeKey]bool{}
			}
			r.taskEscapeSeen[key] = true
		}
		return
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
