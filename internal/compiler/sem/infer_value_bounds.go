package sem

import "kigumi/internal/compiler/syntax"

// valueInstData is a method or fn item value's callee and own-generic substitution. A direct call proves each argument
// (called); any other use escapes, as escapeUse says; bound marks a value a local holds, seen one the escape walk met.
type valueInstData struct {
	m           EntityID
	subst       map[EntityID]TypeID
	called      bool
	bound, seen bool
	escapeAt    syntax.NodeID
	escapeUse   string
}

// recordValueExpr remembers the value at n, its instantiation and the bound checks it made, for the proof at direct
// calls; explicit type arguments leave no variable, so the checks hold the value itself.
func (c *checker) recordValueExpr(n syntax.NodeID, m EntityID, inst []TypeID, recv TypeID, from int) {
	if from == len(c.constraints) {
		return
	}
	d := &valueInstData{m: m, subst: c.callSubst(m, inst, recv)}
	for i := from; i < len(c.constraints); i++ {
		c.constraints[i].held = d
	}
	if c.valueExprs == nil {
		c.valueExprs = map[syntax.NodeID]*valueInstData{}
	}
	c.valueExprs[c.t.Unparen(n)] = d
}

// escapeHolderUses walks the body once, flow-insensitively, treating every reference to a holder as an escape except a
// direct call's callee (proved against every value it can hold) and an alias recorded by noteHolderValue.
func (c *checker) escapeHolderUses(body syntax.NodeID) {
	if len(c.holderVals) == 0 && len(c.valueExprs) == 0 {
		return
	}
	c.resolveHolders()
	c.escapeHolderWalk(body, map[syntax.NodeID]bool{})
}

func (c *checker) escapeHolderWalk(n syntax.NodeID, ok map[syntax.NodeID]bool) {
	t := c.t
	if d := c.valueExprs[n]; d != nil {
		d.seen = true
		if !ok[n] && !d.bound {
			c.escapeValueData(n, d, usePassed)
		}
	}
	switch t.Kind(n) {
	case syntax.Block:
		b := t.View(n).AsBlock()
		for i := range b.Len() - 1 {
			if b.At(i).Kind() == syntax.ExprStmt {
				ok[t.Unparen(b.At(i).AsExprStmt().Expr().ID())] = true
			}
		}
	case syntax.LetStmt:
		if v := t.View(n).AsLet(); v.Pattern().Kind() == syntax.PatWildcard && v.Init().Valid() {
			ok[t.Unparen(v.Init().ID())] = true
		}
	case syntax.Lambda:
		if ent := c.info.Defs[n]; ent != 0 {
			saved := c.walkOwner
			c.walkOwner = ent
			defer func() { c.walkOwner = saved }()
		}
	case syntax.CallExpr:
		v := t.View(n).AsCall()
		ok[t.Unparen(v.Callee().ID())] = true
		c.proveHolderCall(v.Callee().ID(), c.callArgs[n])
	case syntax.WsCallExpr:
		v := t.View(n).AsWsCall()
		ok[t.Unparen(v.Callee().ID())] = true
		c.proveHolderCall(v.Callee().ID(), c.callArgs[n])
	case syntax.Binary:
		if c.info.Calls[n].Piped != 0 {
			rhs := t.View(n).AsBinary().Right().ID()
			ok[t.Unparen(rhs)] = true
			c.proveHolderCall(rhs, c.callArgs[n])
		}
	case syntax.AssignStmt:
		if target := t.Unparen(t.View(n).AsAssign().Target().ID()); t.Kind(target) == syntax.Ident {
			ok[target] = true
		}
	case syntax.Ident:
		if !ok[n] && !c.holderAliasNode[n] {
			if r := c.holderOf(c.info.Uses[n]); r != nil {
				c.escapeHolder(n, r, usePassed)
			}
		}
	}
	for child := range t.View(n).Children() {
		c.escapeHolderWalk(child.ID(), ok)
	}
}

// recordCallArgs keeps a value call's arguments as checkArgs placed them (a piped operand at its `_`, parameter order),
// so the holder walk proves exactly what the call receives.
func (c *checker) recordCallArgs(n syntax.NodeID, args []syntax.NodeID) {
	if c.callArgs == nil {
		c.callArgs = map[syntax.NodeID][]syntax.NodeID{}
	}
	c.callArgs[n] = args
}

// proveHolderCall proves a direct call through callee against each method the holder can hold, once per method, or
// escapes them at the callee when the holder is ever unknown; a value expression called in place is proved directly.
func (c *checker) proveHolderCall(callee syntax.NodeID, args []syntax.NodeID) {
	cu := c.t.Unparen(callee)
	if d := c.valueExprs[cu]; d != nil {
		c.proveValue(d, args)
		d.called = true
		return
	}
	r := c.holderOf(c.info.Uses[cu])
	if r == nil {
		return
	}
	if r.poison {
		c.escapeHolder(cu, r, useMixed)
		return
	}
	for _, d := range r.reps {
		c.proveValue(d, args)
	}
	if !r.called {
		r.called = true
		for _, d := range r.vals {
			d.called = true
		}
	}
}

func (c *checker) proveValue(d *valueInstData, args []syntax.NodeID) {
	sig := c.r.Fn(d.m).Sig
	for i := range args {
		if decl, inner := c.declaredArg(sig, args, i); decl != 0 {
			c.slotContractsMode(inner, decl, d.subst, slotRoot, true)
		}
	}
}
