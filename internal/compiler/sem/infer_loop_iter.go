package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) iterElement(loop, n syntax.NodeID, iter TypeID) TypeID {
	tt := c.r.Types
	if iter == TyPoison {
		return TyPoison
	}
	if tt.Kind(iter) == KUntyped {
		iter = c.adopt(c.literalNode(n), defaultOf(iter))
	}
	// Element keeps the head's borrow, as &T/&mut T bind an argument.
	if tt.Kind(iter) == KRef {
		if inner := c.vars.resolve(tt.Node(iter).Elem); c.isRangeType(n, inner) {
			c.errAt(n, cNotIterable, iter)
			return TyPoison
		} else if tt.Kind(inner) == KNamed && c.memberSetHere(inner, "iter") != 0 {
			c.errAt(n, cIterBorrowedHead, iter)
			return TyPoison
		}
		mut := tt.Node(iter).Flags&flagMut != 0
		elem := c.iterElement(loop, n, c.vars.resolve(tt.Node(iter).Elem))
		if elem == TyPoison {
			return TyPoison
		}
		return tt.Ref(elem, mut)
	}
	if iter == TyString {
		c.errAt(n, cForString)
		return TyPoison
	}
	if iter == TyBytes {
		c.info.Calls[loop] = CallInfo{Kind: CallIter}
		return TyU8
	}
	node := tt.Node(iter)
	if node.Kind == KNamed && (node.Ent == tt.arrayEnt || c.r.isFixedArray(iter)) {
		c.info.Calls[loop] = CallInfo{Kind: CallIter}
		return node.Args[0]
	}
	if c.isRangeType(n, iter) {
		if arg := c.vars.resolve(node.Args[0]); !tt.IsInteger(arg) && !c.vars.pendingLiteral(arg) && tt.Kind(arg) != KUntyped {
			c.errAt(n, cNotIterable, iter)
			return TyPoison
		}
		c.info.Calls[loop] = CallInfo{Kind: CallIter}
		return node.Args[0]
	}
	if me := c.r.mapEntity(); node.Kind == KNamed && me != 0 && node.Ent == me {
		tup, _ := tt.tupleEntity(2)
		c.info.Calls[loop] = CallInfo{Kind: CallIterMap, Callee: me}
		return tt.Named(tup, []TypeID{node.Args[0], node.Args[1]})
	}
	if item, handled := c.iterProtocol(loop, n, iter); handled {
		return item
	}
	if node.Kind == KNamed {
		c.errAtHelp(n, "define `fn iter(self) -> I` on `"+c.r.TypeString(iter)+"`, where `I.next(mut self)` returns `Item?`", cNotIterable, iter)
	} else {
		c.errAt(n, cNotIterable, iter)
	}
	return TyPoison
}

// iterProtocol resolves the name-based `for` protocol: `iter(self) -> I` on
// the head, then `I.next(mut self) -> Item?`. handled is false only when the
// head has no visible `iter`, so the caller can report it as not iterable.
func (c *checker) iterProtocol(loop, n syntax.NodeID, head TypeID) (item TypeID, handled bool) {
	tt := c.r.Types
	if tt.Kind(head) != KNamed {
		return TyPoison, false
	}
	iter, ok, reported := c.protocolMethod(n, head, "iter", RecvSelf, Code{}, cIterAmbiguous, cIterReceiver, cIterArity)
	if !ok {
		return TyPoison, reported
	}
	info := c.r.Fn(iter)
	if len(c.ownGenerics(iter)) > 0 {
		c.errAt(n, cIterGeneric, c.r.Entities[iter].Name)
		return TyPoison, true
	}
	if info.Declared&EffAsync != 0 || len(info.Witnesses) > 0 || len(info.ConstParams) > 0 {
		c.errAt(n, cNotSupportedYet, "a `for` iterator whose `iter` is async or takes hidden arguments")
		return TyPoison, true
	}
	if !c.r.useEntity(c.f, n, iter) {
		return TyPoison, true
	}
	sig, inst, bad := c.instantiateFn(n, iter, nil, head)
	if bad {
		return TyPoison, true
	}
	c.callInvalidates()
	c.edge(EffectEdge{Kind: EdgeCall, Target: iter, Node: n})
	iterType, _ := c.checkArgs(n, c.r.Entities[iter].Name, sig, nil, inst, 0, info.Params, info.Body != 0, false)
	c.crossRecv(n, iter, head, n)
	iterType = c.vars.resolve(iterType)
	if iterType == TyPoison {
		return TyPoison, true
	}
	if tt.Kind(iterType) == KRef {
		c.errAt(n, cIterBorrow, c.r.Entities[iter].Name, iterType)
		return TyPoison, true
	}
	next, ok, _ := c.protocolMethod(n, iterType, "next", RecvMut, cNextMissing, cNextAmbiguous, cNextReceiver, cNextArity)
	if !ok {
		return TyPoison, true
	}
	ninfo := c.r.Fn(next)
	if len(c.r.ownerParams(next)) > 0 {
		c.errAt(n, cNotSupportedYet, "a `for` iterator whose `next` is a method of a generic iterator type")
		return TyPoison, true
	}
	if len(c.ownGenerics(next)) > 0 {
		c.errAt(n, cIterGeneric, c.r.Entities[next].Name)
		return TyPoison, true
	}
	if ninfo.Declared&EffAsync != 0 || len(ninfo.Witnesses) > 0 || len(ninfo.ConstParams) > 0 {
		c.errAt(n, cNotSupportedYet, "a `for` iterator whose `next` is async or takes hidden arguments")
		return TyPoison, true
	}
	if !c.r.useEntity(c.f, n, next) {
		return TyPoison, true
	}
	nsig, ninst, nbad := c.instantiateFn(n, next, nil, iterType)
	if nbad {
		return TyPoison, true
	}
	c.callInvalidates()
	c.edge(EffectEdge{Kind: EdgeCall, Target: next, Node: n})
	ret, _ := c.checkArgs(n, c.r.Entities[next].Name, nsig, nil, ninst, 0, ninfo.Params, ninfo.Body != 0, false)
	c.crossRecv(n, next, iterType, 0)
	ret = c.vars.resolve(ret)
	item, ok = tt.IsOption(ret)
	if !ok {
		c.errAt(n, cNextResult, ret)
		return TyPoison, true
	}
	item = c.vars.resolve(item)
	if tt.Kind(item) == KRef {
		c.errAt(n, cIterItemBorrow, item)
		return TyPoison, true
	}
	c.info.Calls[loop] = CallInfo{Kind: CallIterProtocol, Callee: iter, Next: next, Inst: inst, Recv: RecvSelf, IterType: iterType}
	// The loop owns the iterator state and drops it on every exit; a
	// side-effecting drop must reach the pure/noalloc summaries even when
	// the body only reads the items.
	if !c.r.isCopy(iterType) {
		c.edge(EffectEdge{Kind: EdgeDrop, Type: iterType, Node: n})
	}
	return item, true
}

// protocolMethod picks the single visible zero-argument method `name` on recv
// with the loop's required receiver kind. reported distinguishes a missing
// member (the caller may fall back) from a member that cannot serve the loop.
func (c *checker) protocolMethod(n syntax.NodeID, recv TypeID, name string, want RecvKind, missCode, ambigCode, recvCode, arityCode Code) (ent EntityID, ok, reported bool) {
	set := c.memberSetHere(recv, name)
	if set == 0 {
		if missCode.Num != "" {
			c.errAt(n, missCode, recv)
			return 0, false, true
		}
		return 0, false, false
	}
	var methods, zero []EntityID
	for _, m := range c.visibleMembers(set) {
		info := c.r.Fn(m)
		if info.Recv == RecvNone {
			continue
		}
		methods = append(methods, m)
		sig := c.r.Types.Node(info.Sig)
		if len(sig.Args) == 0 && sig.Flags&(fnVariadic|fnCVariadic) == 0 {
			zero = append(zero, m)
		}
	}
	switch {
	case len(methods) == 0:
		c.errAt(n, recvCode)
		return 0, false, true
	case len(zero) == 0:
		c.errAt(n, arityCode)
		return 0, false, true
	case len(zero) > 1:
		c.errAt(n, ambigCode, recv)
		return 0, false, true
	}
	m := zero[0]
	if c.r.Fn(m).Recv != want {
		c.errAt(n, recvCode)
		return 0, false, true
	}
	return m, true, false
}
