package sem

import "kigumi/internal/syntax"

// Types `v.m` without a call: a fn value copying a Copy receiver, or a closure taking a move-only one.
func (c *checker) methodValue(n, base syntax.NodeID, valueType TypeID, m EntityID, viaRef bool) TypeID {
	info := c.r.Fn(m)
	c.info.Uses[n] = m
	c.info.Calls[n] = CallInfo{Kind: CallMethodValue, Callee: m, Recv: info.Recv}
	sig := c.r.memberSig(valueType, m)
	c.rejectHiddenArgs(n, m)
	async := info.Declared&EffAsync != 0
	owned := info.Recv == RecvSelf && !c.r.isCopy(valueType)
	if info.Recv == RecvMut && !viaRef && c.r.isCopy(valueType) {
		c.errAt(n, cMethodValueCopiedRecv, c.r.Entities[m].Name)
		return TyPoison
	}
	var inst []TypeID
	switch {
	case len(c.ownGenerics(m)) > 0:
		if !c.copyableBoundValue(n, m, valueType) || !c.ownedSync(n, owned && async, valueType) {
			return TyPoison
		}
		shape := sig
		sig, inst, _ = c.instantiateFn(n, m, nil, valueType)
		c.markDeferrable(inst, shape)
		c.touched = append(c.touched, n)
		c.bindMethodWitnesses(n, m, valueType, inst)
	case len(info.Witnesses) > 0 && c.bindableMethodValue(m):
		if !c.ownedSync(n, owned && async, valueType) || !c.bindOwnerBoundValue(n, m, valueType) {
			return TyPoison
		}
	case !c.r.isCopy(valueType) && (!owned || async):
		c.errAt(n, cMethodValueMoveOnly, valueType)
		return TyPoison
	}
	if owned && !c.takeReceiver(n, base, valueType, viaRef) {
		return TyPoison
	}
	c.deferFFIValueCheck(n, m, inst, valueType)
	c.deferTruncateValue(n, m, inst)
	if async {
		sig = c.asyncValueFn(sig)
	} else {
		c.valueEntry(n, m, sig)
	}
	c.buildEdge(n, sig)
	c.edge(EffectEdge{Kind: EdgeCall, Target: m, Node: n})
	c.recordCtxValueRef(n, m)
	if owned {
		return c.ownedMethodValue(n, base, valueType, sig, m)
	}
	return sig
}

func (c *checker) ownedSync(n syntax.NodeID, asyncOwned bool, recv TypeID) bool {
	if asyncOwned {
		c.errAt(n, cMethodValueAsync, recv)
	}
	return !asyncOwned
}

func (c *checker) ownedMethodValue(n, base syntax.NodeID, recv, sig TypeID, m EntityID) TypeID {
	tt := c.r.Types
	c.consumeWhole(base)
	creator := c.fn
	if depth := len(c.lambdas); depth > 0 {
		creator = c.lambdas[depth-1].ent
	}
	key := methodClosureKey{method: m, creator: creator, recv: c.vars.resolve(recv), scoped: c.allocDepth > 0}
	name := c.receiverName(base)
	if id := c.sharedMethodClosure(key, sig, name); id != 0 {
		return tt.Closure(id)
	}
	tok := c.t.View(n).Tok().Index()
	id := c.r.newEntity(Entity{Kind: EntClosure, Name: "closure", Pkg: c.pkg, File: c.f, Node: n, Tok: tok, Parent: c.fn})
	local := c.r.newEntity(Entity{Kind: EntLocal, Name: name, Pkg: c.pkg, File: c.f, Node: base, Tok: tok, Parent: id, Type: recv})
	c.r.Entities[local].Detail = c.r.addLocal(LocalInfo{Scope: c.scope})
	call := EffectEdge{Kind: EdgeCall, Target: m, Node: n, File: c.f, Scoped: key.scoped}
	c.r.Entities[id].Detail = c.r.addClosure(ClosureInfo{
		Ret:      tt.Node(sig).Elem,
		Sig:      sig,
		Captures: []Capture{{Local: local, Mode: capMove}},
		Edges:    []EffectEdge{call},
		Creator:  creator,
		Scoped:   key.scoped,
		Method:   m,
	})
	c.closures = append(c.closures, id)
	if c.methodValues.shared == nil {
		c.methodValues.shared = map[methodClosureKey][]EntityID{}
	}
	c.methodValues.shared[key] = append(c.methodValues.shared[key], id)
	return tt.Closure(id)
}

// A method value's own type arguments come from the Fn bound it is checked against, as a lambda's parameters do.
func (c *checker) hintMethodValue(got, want TypeID) {
	n := c.r.Types.Node(c.vars.resolve(got))
	if n.Kind != KClosure || !c.r.closure(n.Ent).IsMethodValue() {
		return
	}
	if target := c.lambdaTarget(want); target != 0 {
		c.sameFnShape(c.r.closure(n.Ent).Sig, target)
	}
}
