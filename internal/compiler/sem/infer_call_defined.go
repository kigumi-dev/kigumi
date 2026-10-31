package sem

import "kigumi/internal/compiler/syntax"

// callDefinedWrap checks `A(value)`, a nominal defined type's package-private
// representation constructor: one positional argument, no named/default/spread form.
func (c *checker) callDefinedWrap(n syntax.NodeID, head headRef, args []syntax.NodeID, want TypeID) TypeID {
	ent := head.ent
	name := c.r.Entities[ent].Name
	info := c.r.typeDecl(ent)
	typeArgs := c.adtArgs(n, ent, head.args)
	result := c.r.Types.Named(ent, typeArgs)
	if want != 0 {
		c.unifyWant(n, result, want)
	}
	if !c.r.definedAccessible(c.f, ent) {
		c.errAt(n, cDefinedWrapOutside, name, name)
		c.synthRejectedArgs(args)
		return result
	}
	if info.Underlying == TyPoison {
		c.synthRejectedArgs(args)
		return result
	}
	spread := len(args) > 0 && c.t.Kind(args[len(args)-1]) == syntax.Spread
	if c.hasNamedArg(args) || spread {
		c.errAt(n, cDefinedWrapNamedArg, name)
		c.synthRejectedArgs(args)
		return result
	}
	if len(args) != 1 {
		c.errAt(n, cDefinedWrapArity, name, len(args))
		c.synthRejectedArgs(args)
		return result
	}
	subst := instSubst(info.Params, typeArgs)
	underlying := c.r.Types.Subst(info.Underlying, subst)
	c.checkArg(args[0], underlying, 0, false)
	// A defined type's own value is boxed, so a direct underlying value crosses into it.
	c.crossDirect(args[0], underlying)
	c.info.Calls[n] = CallInfo{Kind: CallDefinedWrap, Callee: ent, Inst: typeArgs, Plan: identityPlan(args)}
	c.touched = append(c.touched, n)
	return result
}

// A rejected constructor gives its arguments nothing to infer their open variables from.
func (c *checker) synthRejectedArgs(args []syntax.NodeID) {
	start := c.vars.count()
	c.synthArgs(args)
	c.vars.poisonFrom(start)
}
