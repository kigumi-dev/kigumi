package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) synthMember(n syntax.NodeID) TypeID {
	member := c.t.View(n).AsMember()
	name := member.Member().Text()
	base := member.Base().ID()
	head := c.resolveHead(base)
	switch head.kind {
	case headPkg:
		member := c.packageMember(n, head.pkg, name)
		return c.headValue(n, member)
	case headType:
		member := c.typeMember(n, head, name)
		return c.headValue(n, member)
	case headNone:
		return TyPoison
	}
	recv := c.vars.resolve(c.synthPlace(base))
	c.rejectJoinBase(base)
	return c.memberOn(n, base, recv, name)
}

func (c *checker) headValue(n syntax.NodeID, h headRef) TypeID {
	switch h.kind {
	case headFn:
		return c.fnItemValue(n, h.set)
	case headSet:
		c.errAt(n, cOverloadValueAmbig, c.r.Overloads[h.set].Name)
		return TyPoison
	case headVariant:
		return c.variantValue(n, h.ent, h.args)
	case headType:
		c.errAt(n, cTypeAsValue, c.r.Entities[h.ent].Name)
	case headPkg:
		c.errAt(n, cPackageAsValue, c.r.Entities[h.ent].Name, c.r.Entities[h.ent].Name)
	case headConstraint:
		c.errAt(n, cConstraintAsType, c.r.Entities[h.ent].Name)
	case headValue:
		if h.ent != 0 {
			e := &c.r.Entities[h.ent]
			c.info.Uses[n] = h.ent
			if e.Kind == EntContextKey {
				return c.contextKeyValue(n, h.ent)
			}
			return e.Type
		}
	}
	return TyPoison
}

func (c *checker) memberOn(n, base syntax.NodeID, recv TypeID, name string) TypeID {
	tt := c.r.Types
	if recv == TyPoison {
		return TyPoison
	}
	node := tt.Node(recv)
	viaRef := node.Kind == KRef
	if viaRef {
		recv = node.Elem
		node = tt.Node(recv)
	}
	if node.Kind == KNamed {
		if name == "value" && c.r.typeDecl(node.Ent).Form == FormDefined {
			return c.definedValue(n, node)
		}
		if fld := c.r.findField(node.Ent, name); fld != 0 {
			if !c.r.useEntity(c.f, n, fld) {
				return TyPoison
			}
			c.info.Uses[n] = fld
			c.placeOf(n)
			return c.fieldType(recv, fld)
		}
	}
	if set := c.memberSetHere(recv, name); set != 0 {
		members := c.visibleMembers(set)
		for _, m := range members {
			c.r.useEntity(c.f, n, m)
		}
		if len(members) != 1 {
			c.errAt(n, cOverloadValueAmbig, name)
			return TyPoison
		}
		m := members[0]
		if c.r.Fn(m).Recv == RecvNone {
			c.errAt(n, cMethodAsAssoc, name, c.r.Entities[node.Ent].Name, name)
			return TyPoison
		}
		return c.methodValue(n, base, recv, m, viaRef)
	}
	if req, sig, _, ambiguous := c.ifaceMember(n, recv, name); req != 0 {
		c.info.Uses[n] = req
		call := CallInfo{Kind: CallMethodValue, Callee: req, Recv: c.r.Fn(req).Recv}
		c.recordCtxValueRef(n, req)
		// A `mut self` witness dispatches through a call site's write-back cell; a bare value has none.
		if c.r.Fn(req).Recv == RecvMut {
			c.info.Calls[n] = call
			c.errAt(n, cRequirementMethodValue, name)
			return TyPoison
		}
		if call.Recv == RecvMove && !c.r.isCopy(recv) {
			c.info.Calls[n] = call
			c.errAt(n, cRequirementMoveValue, name, recv)
			return TyPoison
		}
		owned := call.Recv == RecvSelf && !c.r.isCopy(recv)
		if owned && (!c.ownedSync(n, c.r.Fn(req).Declared&EffAsync != 0, recv) || !c.takeReceiver(n, base, recv, viaRef)) {
			c.info.Calls[n] = call
			return TyPoison
		}
		if node.Kind == KParam {
			call.Witness = c.witnessLocal(n, node.Ent, req)
		}
		c.info.Calls[n] = call
		if owned {
			return c.ownedMethodValue(n, base, recv, sig, req)
		}
		return sig
	} else if ambiguous {
		return TyPoison
	}
	if node.Kind == KParam {
		c.errAt(n, cParamNoMember, recv, name)
		return TyPoison
	}
	if pkg := c.hiddenMemberPackage(recv, name); pkg != "" {
		c.errAtHelp(n, hiddenMemberHelp(name, pkg), cMemberNotFound, c.foreignTypeArg(recv), name)
		return TyPoison
	}
	c.errSuggest(n, cMemberNotFound, name, c.r.memberNames(c.r.namedEnt(recv)), c.foreignTypeArg(recv), name)
	return TyPoison
}

func (c *checker) fieldType(recv TypeID, fld EntityID) TypeID {
	node := c.r.Types.Node(recv)
	owner := c.r.Entities[fld].Parent
	params := c.r.typeDecl(owner).Params
	if len(params) == 0 {
		return c.r.Entities[fld].Type
	}
	subst := map[EntityID]TypeID{}
	for i, p := range params {
		if i < len(node.Args) {
			subst[p] = node.Args[i]
		}
	}
	return c.r.Types.Subst(c.r.Entities[fld].Type, subst)
}
