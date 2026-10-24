package sem

import "kigumi/internal/syntax"

// Expected type is unified before the payload so it can resolve typeArgs.
func (c *checker) callVariant(n syntax.NodeID, head headRef, args []syntax.NodeID, want TypeID) TypeID {
	v := head.ent
	vi := c.r.variant(v)
	name := c.r.Entities[v].Name
	c.info.Uses[c.t.View(n).AsCall().Callee().ID()] = v
	if len(vi.Payload) == 0 {
		c.errFix(n, c.replaceNode(n, "Write `"+name+"` without parentheses", name), cVariantNoPayload, name, name)
		c.synthArgs(args)
		return TyPoison
	}
	adt := c.r.Entities[v].Parent
	typeArgs := c.adtArgs(n, adt, head.args)
	result := c.r.Types.Named(adt, typeArgs)
	if want != 0 {
		c.unifyWant(n, result, want)
	}
	if len(args) != len(vi.Payload) {
		c.errAt(n, cVariantArity, name, len(vi.Payload), plural(len(vi.Payload)), len(args))
		c.synthArgs(c.unwrapNamed(args))
		return result
	}
	plan := identityPlan(args)
	if c.hasNamedArg(args) {
		lay := c.layoutNamed(args, vi.Names)
		if !lay.ok() {
			c.reportNamedLayout(name, lay)
			c.synthArgs(c.unwrapNamed(args))
			return result
		}
		plan = newCallPlan(c.unwrapNamed(args), lay.place())
		args = plan.placed()
	}
	subst := map[EntityID]TypeID{}
	for i, p := range c.r.typeDecl(adt).Params {
		subst[p] = typeArgs[i]
	}
	for i, a := range args {
		if c.t.Kind(a) == syntax.Spread {
			c.errAt(a, cSpreadNonVariadic, name)
			continue
		}
		payload := c.r.Types.Subst(vi.Payload[i], subst)
		c.checkArg(a, payload, 0, false)
		c.rejectScalarCopyStore(a, payload)
		if c.t.Kind(c.t.Unparen(a)) == syntax.MemberExpr {
			c.rejectFieldValue(a)
		}
		c.checkEscapeStore(a)
	}
	if !c.fixedPayload(args) {
		c.buildEdge(n, result)
	}
	c.info.Calls[n] = CallInfo{Kind: CallVariant, Callee: v, Inst: typeArgs, Plan: plan}
	c.touched = append(c.touched, n)
	return result
}

func (c *checker) callMethod(n, callee syntax.NodeID, args []syntax.NodeID, want TypeID) TypeID {
	member := c.t.View(callee).AsMember()
	base := member.Base().ID()
	name := member.Member().Text()
	recv := c.vars.resolve(c.synthPlace(base))
	tt := c.r.Types
	if recv == TyPoison {
		c.synthArgs(args)
		return TyPoison
	}
	recv, valueType, ok := c.settleReceiver(base, recv, name)
	if !ok {
		c.synthArgs(args)
		return TyPoison
	}
	if tt.Kind(valueType) == KNamed {
		if fld := c.r.findField(tt.Node(valueType).Ent, name); fld != 0 {
			ft := c.memberOn(callee, base, recv, name)
			c.setType(callee, ft)
			resolved := c.vars.resolve(ft)
			if fact, ok := c.knownFieldEffect(callee); ok {
				if fact.Param != 0 || fact.Target != 0 {
					c.stashFieldFact(n, fact)
				} else {
					resolved = c.upgradeFnEffects(resolved, fact.Mods)
				}
			} else if p := c.aliasedParam(base); p != 0 {
				c.deferToFieldParam(p, fld)
				c.stashFieldFact(n, NarrowFact{Kind: FactPureField, Param: p})
				c.edge(EffectEdge{Kind: EdgeKnownCall, Target: c.r.fieldParamEnt(fieldFnKey{Ent: p, Field: fld}), Node: n})
			} else if place, ok := c.nestedParamPlace(base); ok {
				c.nestedFieldCall(n, place, fld)
			}
			c.copiedPlaceCalls(n, base, fld)
			return c.callValue(n, callee, resolved, args, want)
		}
	}
	return c.callMethodOn(n, callee, base, recv, valueType, nil, args, want)
}

func (c *checker) callMethodOn(n, callee, base syntax.NodeID, recv, valueType TypeID, typeArgs []TypeID, args []syntax.NodeID, want TypeID) TypeID {
	tt := c.r.Types
	name := c.t.View(callee).Tok().Text()
	set := c.memberSetHere(valueType, name)
	if set == 0 {
		if req, sig, iface, ambiguous := c.ifaceMember(callee, valueType, name); req != 0 {
			return c.callRequirement(n, callee, base, recv, req, sig, iface, typeArgs, args, want)
		} else if ambiguous {
			c.synthArgs(args)
			return TyPoison
		}
		c.synthArgs(args)
		if name == "display" && len(args) == 0 && c.requestDisplay(valueType) || c.requestProtoMethod(valueType, name, len(args)) {
			return TyPoison
		}
		if tt.Kind(valueType) == KParam {
			c.errAt(callee, cParamNoMember, valueType, name)
		} else if pkg := c.hiddenMemberPackage(valueType, name); pkg != "" {
			c.errAtHelp(callee, hiddenMemberHelp(name, pkg), cMemberNotFound, valueType, name)
		} else {
			c.errSuggest(callee, cMemberNotFound, name, c.r.memberNames(c.r.namedEnt(valueType)), valueType, name)
		}
		return TyPoison
	}
	members := c.visibleMembers(set)
	m := members[0]
	if len(members) == 1 {
		c.r.useEntity(c.f, callee, m)
	} else if m = c.pickOverload(n, set, args, valueType); m == 0 {
		return TyPoison
	}
	info := c.r.Fn(m)
	c.noteDerivedDisplayUse(m)
	if info.Recv == RecvNone {
		c.synthArgs(args)
		c.errAt(callee, cMethodAsAssoc, name, c.r.entityName(tt.Node(valueType).Ent), name)
		return TyPoison
	}
	c.info.Uses[callee] = m
	c.checkReceiver(base, recv, m)
	result := c.callFn(n, m, typeArgs, args, want, valueType, base)
	if info.Recv == RecvMut {
		c.checkReentrantCapture(base, args)
	}
	c.noteRecvStores(n, base, m, args)
	c.rejectLentRecv(base, args)
	c.checkSelfAliasArgs(base, info.Recv, info.Declared&EffAsync != 0, args)
	c.noteBorrowedRecv(base, recv, m)
	c.noteSelfRecvCall(base, m, false, 0)
	return result
}

func (c *checker) checkReceiver(base syntax.NodeID, recv TypeID, m EntityID) {
	info := c.r.Fn(m)
	name := c.r.Entities[m].Name
	tt := c.r.Types
	switch info.Recv {
	case RecvMut:
		if tt.Kind(recv) == KRef && tt.Node(recv).Flags&flagMut != 0 {
			c.rejectSharedMutRef(base, recv)
			return
		}
		if c.rootIsChainHole(base) {
			c.errAt(base, cRecvTemporary, name)
		} else if c.indexAssignUnsupported(base) {
			c.errAt(base, cIndexAssignUnsupported)
		} else if !c.isPlace(base) {
			c.errAt(base, cRecvTemporary, name)
		} else if c.rejectCtxKeyWrite(base, "the receiver of `mut self` method `"+name+"`") {
		} else if !c.isMutablePlace(base) {
			c.reportRecvNotMutable(base, name)
		} else if root := c.rootOf(base); root != 0 {
			c.checkCapturedMutConflict(base, root)
		}
		c.reserveMutRecv(base, m)
		c.reserveMutDrain(base, m)
		c.invalidatePlace(base)
	case RecvMove:
		if tt.Kind(recv) == KRef {
			c.errAt(base, cMoveOutOfBorrow, tt.Node(recv).Elem)
		} else if c.indexRooted(base) && !c.copyNow(recv) {
			c.errAtHelp(base, c.noCloneHelp(recv, "borrow it with `&`, or take it with `.takeAt`"), cIndexValueCopy, recv)
		} else {
			c.consumeWhole(base)
		}
	}
}
