package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// fieldForwardEdge: case-(b) forwarding relation.
type fieldForwardEdge struct {
	Caller, CallerParam, Callee, CalleeParam EntityID
}

// closeFieldParamForwarding pre-seeds CalledFieldParams to a fixpoint over
// the whole call graph: declaration order alone would miss a `pure?` field
// forwarded through more than one function before its callee is checked.
func (r *Result) closeFieldParamForwarding() {
	var edges []fieldForwardEdge
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind != EntFn || e.Flags&EfPoison != 0 || e.File == 0 {
			continue
		}
		info := r.Fn(EntityID(id))
		if info.Body == 0 || r.Entities[e.Parent].Kind == EntInterface {
			continue
		}
		edges = append(edges, r.scanFieldForwarding(EntityID(id), e.File, info)...)
	}
	for changed := true; changed; {
		changed = false
		for _, fe := range edges {
			for _, key := range r.Fn(fe.Callee).CalledFieldParams {
				if key.Param == fe.CalleeParam && r.addCalledFieldParam(fe.Caller, fe.CallerParam, key.Field) {
					changed = true
				}
			}
		}
	}
}

func (r *Result) addCalledFieldParam(fn, param, field EntityID) bool {
	info := r.Fn(fn)
	key := FieldParamKey{Param: param, Field: field}
	r.markDeferred(r.fieldParamEnt(fieldFnKey{Ent: param, Field: field}), false)
	for _, k := range info.CalledFieldParams {
		if k == key {
			return false
		}
	}
	info.CalledFieldParams = append(info.CalledFieldParams, key)
	return true
}

func (r *Result) scanFieldForwarding(fn EntityID, file FileID, info *FnInfo) []fieldForwardEdge {
	var edges []fieldForwardEdge
	t := r.tree(file)
	scope := r.declScope(fn)
	var walk func(n syntax.NodeID)
	walk = func(n syntax.NodeID) {
		if n == 0 || t.Kind(n) == syntax.Lambda {
			return
		}
		if t.Kind(n) == syntax.CallExpr || t.Kind(n) == syntax.WsCallExpr {
			r.scanFieldCall(fn, info, t, n, scope, &edges)
		}
		for ch := range t.View(n).Children() {
			walk(ch.ID())
		}
	}
	walk(info.Body)
	return edges
}

func (r *Result) scanFieldCall(fn EntityID, info *FnInfo, t *syntax.Tree, call syntax.NodeID, scope ScopeID, edges *[]fieldForwardEdge) {
	cv := t.View(call).AsCallLike()
	callee := cv.Callee().ID()
	if t.Kind(callee) == syntax.MemberExpr {
		mnode := t.View(callee).AsMember()
		if p := r.paramIdent(info, t, mnode.Base().ID()); p != 0 {
			if fld := r.effectPolyFieldOn(r.Entities[p].Type, mnode.Member().Text()); fld != 0 {
				r.addCalledFieldParam(fn, p, fld)
			}
		}
		return
	}
	if t.Kind(callee) != syntax.Ident {
		return
	}
	args := cv.Args().AppendIDs(nil)
	for _, g := range r.candidateFnBindings(scope, t.View(callee).Tok().Text()) {
		gParams := r.Fn(g).Params
		if r.callArgsDefinitelyMismatch(info, t, args, gParams) {
			continue
		}
		for i, a := range args {
			if i >= len(gParams) || t.Kind(a) == syntax.Spread {
				continue
			}
			if p := r.paramIdent(info, t, a); p != 0 {
				*edges = append(*edges, fieldForwardEdge{Caller: fn, CallerParam: p, Callee: g, CalleeParam: gParams[i]})
			}
		}
	}
}

// paramIdent matches by text instead of real scope tracking: a wrong extra
// key only makes a caller's forwarding check more conservative, never less
// sound.
func (r *Result) paramIdent(info *FnInfo, t *syntax.Tree, n syntax.NodeID) EntityID {
	if t.Kind(n) != syntax.Ident {
		return 0
	}
	name := t.View(n).Tok().Text()
	for _, p := range info.Params {
		if r.Entities[p].Name == name {
			if r.Entities[p].Flags&EfMut != 0 {
				return 0
			}
			return p
		}
	}
	return 0
}

func (r *Result) effectPolyFieldOn(typ TypeID, name string) EntityID {
	if r.Types.Kind(typ) == KRef {
		typ = r.Types.Node(typ).Elem
	}
	if r.Types.Kind(typ) != KNamed {
		return 0
	}
	if fld := r.findField(r.Types.Node(typ).Ent, name); fld != 0 && r.isEffectPolyField(fld) {
		return fld
	}
	return 0
}

// candidateFnBindings over-approximates: this pass runs before
// body-checking, so it can't run pickOverload's full resolution. A wrong
// candidate only makes the caller's forwarding check more conservative,
// never less sound, same as an extra CalledFieldParams key (see paramIdent).
func (r *Result) candidateFnBindings(scope ScopeID, name string) []EntityID {
	b, _, ok := r.lookup(scope, name)
	if !ok {
		return nil
	}
	ent := b.Ent
	if ent == 0 {
		return r.overloadFnMembers(b.Set)
	}
	ent = r.follow(ent)
	if r.Entities[ent].Kind != EntFn {
		return nil
	}
	if set := r.Fn(ent).Set; set != 0 && len(r.Overloads[set].Members) > 1 {
		return r.overloadFnMembers(set)
	}
	return []EntityID{ent}
}

func (r *Result) overloadFnMembers(set OverloadSetID) []EntityID {
	var out []EntityID
	for _, m := range r.Overloads[set].Members {
		if r.Entities[m].Kind == EntFn {
			out = append(out, m)
		}
	}
	return out
}

// markForwardedFieldCalls closes the link a field makes through an ordinary
// fn-typed parameter: when a body hands `self.handler` to a callee parameter
// that the callee calls, the field is forwarded and its own callers must bind
// it, exactly as if the body had called the field itself.
func (r *Result) markForwardedFieldCalls() {
	for changed := true; changed; {
		changed = false
		for id := 1; id < len(r.Entities); id++ {
			e := &r.Entities[id]
			if e.Kind != EntFn || e.Flags&EfPoison != 0 || e.File == 0 {
				continue
			}
			info := r.Fn(EntityID(id))
			if info.Body == 0 {
				continue
			}
			for _, edge := range info.Edges {
				if edge.Kind != EdgeFieldBind || edge.Param == 0 || edge.Target == 0 {
					continue
				}
				caller := r.Entities[edge.Param].Parent
				if caller == 0 || r.Entities[caller].Kind != EntFn || !slices.Contains(r.Fn(caller).CalledParams, edge.Param) {
					continue
				}
				if r.Entities[edge.Target].Kind != EntFieldParam {
					continue
				}
				key, ok := r.fieldKeyByEnt[edge.Target]
				if !ok || r.Entities[key.Ent].Parent != EntityID(id) {
					continue
				}
				if r.addCalledFieldParam(EntityID(id), key.Ent, key.Field) {
					changed = true
				}
			}
			for _, late := range r.lateFieldEdges {
				if late.kind != EdgeFieldBind || late.caller != EntityID(id) || late.param == 0 || r.Entities[late.param].Parent == 0 {
					continue
				}
				caller := r.Entities[late.param].Parent
				if r.Entities[caller].Kind != EntFn || !slices.Contains(r.Fn(caller).CalledParams, late.param) {
					continue
				}
				if r.addCalledFieldParam(EntityID(id), late.src.Ent, late.src.Field) {
					changed = true
				}
			}
		}
	}
}
