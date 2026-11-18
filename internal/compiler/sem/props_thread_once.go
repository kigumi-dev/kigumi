package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// callsOnce: fn calls the value its parameter of type f holds at most once. An Fn or FnMut bound, or any use but one
// consuming call or a relay to a once-calling callee, may call it again.
func (r *Result) callsOnce(fn, f EntityID) bool {
	if fn == 0 || !r.onceBound(f) {
		return false
	}
	info := r.Fn(fn)
	if info.Body == 0 {
		return info.Owner == 0 || r.Entities[info.Owner].Kind != EntInterface
	}
	st := &r.thread
	key := producer{fn: fn, param: f}
	if ok, seen := st.once[key]; seen {
		return ok
	}
	if st.once == nil {
		st.once = map[producer]bool{}
	}
	st.once[key] = false
	p, ok := r.soleParamOf(info, f)
	ok = ok && r.paramCalls(fn, p) <= 1
	st.once[key] = ok
	return ok
}

func (r *Result) onceBound(f EntityID) bool {
	once := false
	for _, con := range r.typeParam(f).Constraints {
		switch con.Kind {
		case CFnOnce:
			once = true
		case CSend, CSync, CIface:
			// A marker interface adds no way to call the value; Copy, Fn and FnMut would let the body call it again.
		default:
			return false
		}
	}
	return once
}

// soleParamOf is the one parameter of type f, when no other parameter's type names f.
func (r *Result) soleParamOf(info *FnInfo, f EntityID) (EntityID, bool) {
	tt := r.Types
	var out EntityID
	params := append([]EntityID{info.SelfParam}, info.Params...)
	for _, p := range params {
		if p == 0 {
			continue
		}
		t := r.Entities[p].Type
		switch {
		case tt.Kind(t) == KParam && tt.Node(t).Ent == f && out == 0:
			out = p
		case slices.Contains(tt.paramsIn(t, nil), f):
			return 0, false
		}
	}
	return out, out != 0
}

// paramCalls counts, over fn's body, the calls through p on the busiest path, or 2 for a use that may call it more
// than once.
func (r *Result) paramCalls(fn, p EntityID) int {
	fi := &r.Files[r.Entities[fn].File]
	roles := map[syntax.NodeID]paramRole{}
	var path pathFn
	path = func(v syntax.View, nested bool) pathCount {
		if !v.Valid() {
			return zeroPath
		}
		switch v.Kind() {
		case syntax.ForExpr:
			return r.loopPathCount(v, nested, path)
		case syntax.Lambda:
			return fallPath(max(0, r.childPathCount(v, true, path).max()))
		}
		r.noteCallRoles(fi, v.ID(), roles)
		self := zeroPath
		if fi.Uses[v.ID()] == p {
			if nested || !r.roleCallsOnce(fi, roles[v.ID()]) {
				self.fall = 2
			} else {
				self.fall = 1
			}
		}
		out := seqCount(self, r.childPathCount(v, nested, path))
		if fi.Types[v.ID()] == TyNever {
			out = out.jump(v.Kind())
		}
		return out
	}
	return path(fi.Tree.View(r.Fn(fn).Body), false).max()
}

func (r *Result) childPathCount(v syntax.View, nested bool, path pathFn) pathCount {
	switch v.Kind() {
	case syntax.IfExpr:
		f := v.AsIf()
		return seqCount(path(f.Cond().View, nested), branchCount(path(f.Then().View, nested), path(f.Else().View, nested)))
	case syntax.IfLetExpr:
		f := v.AsIfLet()
		head := seqCount(path(f.Init().View, nested), path(f.Guard().View, nested))
		return seqCount(head, branchCount(path(f.Then().View, nested), path(f.Else().View, nested)))
	case syntax.MatchExpr:
		return r.matchPathCount(v.AsMatch(), nested, path)
	}
	out := zeroPath
	for child := range v.Children() {
		out = seqCount(out, path(child, nested))
	}
	return out
}

// paramRole is how a call takes an operand: as its callee, or into parameter slot of the fn it names.
type paramRole struct {
	call   syntax.NodeID
	slot   int
	callee bool
}

func (r *Result) noteCallRoles(fi *FileInfo, n syntax.NodeID, roles map[syntax.NodeID]paramRole) {
	ci, ok := fi.Calls[n]
	if !ok {
		return
	}
	t := fi.Tree
	switch t.Kind(n) {
	case syntax.CallExpr:
		roles[t.Unparen(t.View(n).AsCall().Callee().ID())] = paramRole{call: n, callee: true}
	case syntax.WsCallExpr:
		roles[t.Unparen(t.View(n).AsWsCall().Callee().ID())] = paramRole{call: n, callee: true}
	case syntax.Binary:
		if ci.Piped != 0 {
			roles[t.Unparen(t.View(n).AsBinary().Right().ID())] = paramRole{call: n, callee: true}
		}
	}
	if ci.Plan == nil {
		return
	}
	for k, a := range ci.Plan.Eval {
		if k < len(ci.Plan.Order) {
			roles[t.Unparen(a)] = paramRole{call: n, slot: ci.Plan.Order[k]}
		}
	}
}

func (r *Result) roleCallsOnce(fi *FileInfo, role paramRole) bool {
	if role.call == 0 || role.callee {
		return role.callee
	}
	ci := fi.Calls[role.call]
	if ci.Kind != CallFn && ci.Kind != CallAssoc && ci.Kind != CallMethod || ci.Callee == 0 || ci.Witness != 0 {
		return false
	}
	g := r.Fn(ci.Callee)
	if role.slot < 0 || role.slot >= len(g.Params) {
		return false
	}
	t := r.Entities[g.Params[role.slot]].Type
	return r.Types.Kind(t) == KParam && r.callsOnce(ci.Callee, r.Types.Node(t).Ent)
}
