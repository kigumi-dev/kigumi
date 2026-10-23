package sem

import "kigumi/internal/syntax"

// Brackets are ambiguous: `obj.items[0](x)` indexes a field then calls the
// element; otherwise they're a method's explicit type arguments.
func (c *checker) callBracketMethod(n, callee syntax.NodeID, args []syntax.NodeID, want TypeID) (TypeID, bool) {
	br := c.t.View(callee).AsBracket()
	inner := br.Base().ID()
	if c.t.Kind(inner) != syntax.MemberExpr || c.resolveHead(inner).kind != headValue {
		return 0, false
	}
	tt := c.r.Types
	member := c.t.View(inner).AsMember()
	base := member.Base().ID()
	name := member.Member().Text()
	items := br.Args().AppendIDs(nil)
	recv := c.vars.resolve(c.synth(base))
	if recv == TyPoison {
		c.synthArgs(args)
		return TyPoison, true
	}
	recv, valueType, ok := c.settleReceiver(base, recv, name)
	if !ok {
		c.synthArgs(args)
		return TyPoison, true
	}
	isField := tt.Kind(valueType) == KNamed && c.r.findField(tt.Node(valueType).Ent, name) != 0
	if isField || c.memberSetHere(valueType, name) == 0 && !c.hasRequirement(valueType, name) {
		ft := c.vars.resolve(c.memberOn(inner, base, recv, name))
		c.setType(inner, ft)
		if ft == TyPoison {
			c.synthArgs(args)
			return TyPoison, true
		}
		if len(items) != 1 {
			c.errAt(callee, cIndexCount)
			c.synthArgs(args)
			return TyPoison, true
		}
		index := c.vars.resolve(c.synth(items[0]))
		elem := c.indexType(callee, items[0], ft, index)
		c.setType(callee, elem)
		return c.callValue(n, 0, c.vars.resolve(elem), args, want), true
	}
	var params []EntityID
	if members := c.r.Overloads[c.r.memberSet(valueType, name)].Members; len(members) == 1 {
		params = c.ownGenerics(members[0])
	}
	var typeArgs []TypeID
	for i, a := range items {
		typeArgs = append(typeArgs, c.r.resolveExprGenericArg(c.f, c.scope, a, entAt(params, i)))
	}
	return c.callMethodOn(n, inner, base, recv, valueType, typeArgs, args, want), true
}

// settleReceiver fixes the type of a still-untyped literal receiver by the method it names.
func (c *checker) settleReceiver(base syntax.NodeID, recv TypeID, name string) (TypeID, TypeID, bool) {
	tt := c.r.Types
	if c.vars.pendingLiteral(recv) {
		recv = c.vars.settleLiterals(recv)
	}
	valueType := recv
	if tt.Kind(recv) == KRef {
		valueType = tt.Node(recv).Elem
	}
	recv, valueType = c.settlePendingReceiver(recv, valueType, name)
	if tt.Kind(valueType) == KUntyped {
		if lit := c.literalNode(base); c.isConstLiteral(lit) {
			if resolved, ok := c.resolveLiteralReceiver(valueType, name); ok {
				if recv = c.adopt(lit, resolved); recv == TyPoison {
					return recv, valueType, false
				}
				valueType = recv
			}
		}
	}
	return recv, valueType, true
}
