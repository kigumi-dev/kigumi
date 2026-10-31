package sem

import "kigumi/internal/compiler/syntax"

// A call on the right of `|>` takes the left operand first.
func (c *checker) synthCall(n syntax.NodeID, want TypeID) TypeID {
	call := c.t.View(n).AsCallLike()
	callee := call.Callee().ID()
	args := call.Args().AppendIDs(nil)
	if p := c.piped[n]; p != 0 {
		var ok bool
		if args, ok = c.pipeArgs(n, p, args); !ok {
			c.synth(p)
			c.synthArgs(nonHoles(c, call.Args().AppendIDs(nil)))
			return TyPoison
		}
	}
	t := c.synthCallWith(n, callee, args, want)
	if p := c.piped[n]; p != 0 {
		ci := c.info.Calls[n]
		ci.Piped = p
		c.info.Calls[n] = ci
	}
	return t
}

func (c *checker) synthCallWith(n, callee syntax.NodeID, args []syntax.NodeID, want TypeID) TypeID {
	head := c.resolveHead(callee)
	switch head.kind {
	case headFn:
		c.info.Uses[callee] = head.ent
		return c.callFn(n, head.ent, head.args, args, want, 0, 0)
	case headSet:
		m := c.pickOverload(n, head.set, args, 0)
		if m == 0 {
			return TyPoison
		}
		c.info.Uses[callee] = m
		return c.callFn(n, m, head.args, args, want, 0, 0)
	case headVariant:
		return c.callVariant(n, head, args, want)
	case headIntrinsic:
		return c.callIntrinsic(n, head.ent, args)
	case headStatic:
		return c.callStatic(n, callee, head, args, want)
	case headType:
		if c.r.Entities[head.ent].Kind == EntType && c.r.typeDecl(head.ent).Form == FormDefined {
			return c.callDefinedWrap(n, head, args, want)
		}
		c.synthArgs(args)
		if c.r.Entities[head.ent].Kind == EntAlias {
			c.errAt(callee, cAliasNotConstructible, c.r.Entities[head.ent].Name)
			return TyPoison
		}
		c.errAt(callee, cTypeAsValue, c.r.Entities[head.ent].Name)
		return TyPoison
	case headParam:
		c.synthArgs(args)
		c.errAt(callee, cTypeAsValue, c.r.Entities[head.ent].Name)
		return TyPoison
	case headPkg:
		c.synthArgs(args)
		c.errAt(callee, cPackageAsValue, c.r.Entities[head.ent].Name, c.r.Entities[head.ent].Name)
		return TyPoison
	case headNone:
		c.synthArgs(args)
		return TyPoison
	}
	if c.t.Kind(callee) == syntax.MemberExpr {
		return c.callMethod(n, callee, args, want)
	}
	if c.t.Kind(callee) == syntax.BracketExpr {
		if t, ok := c.callBracketMethod(n, callee, args, want); ok {
			return t
		}
	}
	ft := c.vars.resolve(c.synth(callee))
	return c.callValue(n, callee, ft, args, want)
}

func nonHoles(c *checker, args []syntax.NodeID) []syntax.NodeID {
	var out []syntax.NodeID
	for _, a := range args {
		if c.nestedHole(a) == 0 {
			out = append(out, a)
		}
	}
	return out
}
