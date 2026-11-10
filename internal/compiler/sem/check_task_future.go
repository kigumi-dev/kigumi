package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

const (
	futureAwaited = 1 + iota
	futureSpawned
)

// taskFutureCall is a call whose Future holds a `Task`; mut says it can also write one through `&mut` or `mut self`, and
// ifaces that it holds one only through those interfaces.
type taskFutureCall struct {
	n       syntax.NodeID
	t       TypeID
	mut     bool
	ifaces  []TypeID
	futures []TypeID
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

// taskFutureOf lists the parameter types, and for a fn value the argument types, through which the Future holds a `Task`:
// the first held directly, then the first an interface may hide, `&mut` first.
func (r *Result) taskFutureOf(f FileID, n syntax.NodeID, call CallInfo, ret TypeID) []taskFutureCall {
	direct := r.Types.Kind(ret) == KNamed && r.Types.Node(ret).Ent == r.Types.futureEnt
	if !direct && !r.holdsFuture(ret, true) {
		return nil
	}
	structural := direct || r.holdsFuture(ret, false)
	var params []TypeID
	mutSelf := false
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
		mutSelf = info.SelfParam != 0 && info.Recv == RecvMut
	case t.Kind(n) == syntax.CallExpr || t.Kind(n) == syntax.WsCallExpr:
		params = slices.Clone(r.Types.Node(r.Files[f].Types[t.View(n).AsCall().Callee().ID()]).Args)
		for a := range t.View(n).AsCall().Args().IDs() {
			if at := r.Files[f].Types[a]; at != 0 {
				params = append(params, at)
			}
		}
	}
	var found, hidden []taskFutureCall
	for i, p := range params {
		mut, h := true, r.mutRefHolding(p, nil)
		if mutSelf && i == 0 {
			h = r.taskHoldingOf(p)
		}
		if !h.any() {
			mut, h = false, r.taskHoldingOf(p)
		}
		switch {
		case h.hit:
			found = append(found, taskFutureCall{n: n, t: p, mut: mut})
		case len(h.ifaces) > 0:
			hidden = append(hidden, taskFutureCall{n: n, t: p, mut: mut, ifaces: h.ifaces})
		}
	}
	out := append(firstWriting(found), firstWriting(hidden)...)
	if !structural {
		futures := r.taskHoldingOf(ret).ifaces
		for i := range out {
			out[i].futures = futures
		}
	}
	return out
}

func firstWriting(calls []taskFutureCall) []taskFutureCall {
	if i := slices.IndexFunc(calls, func(c taskFutureCall) bool { return c.mut }); i >= 0 {
		return calls[i : i+1]
	}
	return calls[:min(len(calls), 1)]
}

// checkTaskFutures rejects a Future holding a handle unless it is awaited at once, or, when it cannot write one back,
// spawned at once; a call holding one only through an interface waits for its implementers.
func (r *Result) checkTaskFutures(f FileID, calls []taskFutureCall) {
	done := map[syntax.NodeID]bool{}
	for _, c := range calls {
		var where string
		switch how := r.taskFutureUse[taskEscapeKey{f, c.n}]; {
		case done[c.n], how == futureAwaited, how == futureSpawned && !c.mut:
			continue
		case c.mut:
			where = "a write through a `&mut` or `mut self` parameter of a Future that is not awaited where it is built"
		default:
			where = "an argument of a Future that is not awaited or spawned where it is built"
		}
		done[c.n] = true
		if len(c.ifaces) > 0 || len(c.futures) > 0 {
			r.taskHidden = append(r.taskHidden, hiddenTaskSite{f: f, n: c.n, code: cTaskHandleReturned,
				args: []any{r.TypeString(c.t), where}, ifaces: c.ifaces, futures: c.futures})
			continue
		}
		r.errAt(f, c.n, cTaskHandleReturned, r.TypeString(c.t), where)
	}
}

type futureKey struct {
	t     TypeID
	iface bool
}

// holdsFuture is containsFuture from the top of t, kept per type.
func (r *Result) holdsFuture(t TypeID, iface bool) bool {
	k := futureKey{t, iface}
	if v, ok := r.futureHolds[k]; ok {
		return v
	}
	v := r.containsFuture(t, iface, 0, map[EntityID]bool{}, map[TypeID]bool{})
	if r.futureHolds == nil {
		r.futureHolds = map[futureKey]bool{}
	}
	r.futureHolds[k] = v
	return v
}

// containsFuture reports a value type that can carry a Future, inside a record, variant, array or, with iface, an
// interface; it does not substitute type arguments, and the depth cap stops recursive aliases.
func (r *Result) containsFuture(t TypeID, iface bool, depth int, ents map[EntityID]bool, seen map[TypeID]bool) bool {
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
				if r.containsFuture(decl.Underlying, iface, depth+1, ents, seen) {
					return true
				}
			case FormRecord, FormResource:
				for _, fld := range decl.Fields {
					if r.containsFuture(r.field(fld).Type, iface, depth+1, ents, seen) {
						return true
					}
				}
			case FormAdt:
				for _, v := range decl.Variants {
					for _, pt := range r.variant(v).Payload {
						if r.containsFuture(pt, iface, depth+1, ents, seen) {
							return true
						}
					}
				}
			}
			delete(ents, n.Ent)
		}
	}
	if n.Kind == KIface {
		return iface
	}
	if r.containsFuture(n.Elem, iface, depth+1, ents, seen) {
		return true
	}
	for _, a := range n.Args {
		if r.containsFuture(a, iface, depth+1, ents, seen) {
			return true
		}
	}
	return false
}
