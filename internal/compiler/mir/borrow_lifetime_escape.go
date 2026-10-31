package mir

import (
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// paramLifetime is prm's own named lifetime (an explicit `'a` tag, self's sole
// lifetime parameter, or a `T: Peek['a]` bound) and the node to blame on escape.
func (c *borrowCheck) paramLifetime(prm LocalID) (sem.EntityID, syntax.NodeID) {
	ent := c.f.Locals[prm].Ent
	if ent == 0 || c.p.R.Entity(ent).Kind != sem.EntParam {
		return 0, 0
	}
	node := c.p.R.Entity(ent).Node
	if lt := c.p.R.Local(ent).Lifetime; lt != 0 {
		return lt, node
	}
	if c.p.R.Entity(ent).Flags&sem.EfSelf != 0 {
		return c.selfLifetime(), node
	}
	if c.p.R.Entity(c.f.Ent).Kind != sem.EntFn {
		return 0, node
	}
	return c.witnessBoundLifetime(c.p.R.Fn(c.f.Ent), c.f.Locals[prm].Type), node
}

// witnessBoundLifetime is the lifetime a bare type-parameter of info's own
// function carries through an interface bound naming one (`T: Peek['a]`).
func (c *borrowCheck) witnessBoundLifetime(info *sem.FnInfo, typ sem.TypeID) sem.EntityID {
	if c.p.R.Types.Kind(typ) != sem.KParam {
		return 0
	}
	param := c.p.R.Types.Node(typ).Ent
	for _, w := range info.Witnesses {
		if w.Param != param {
			continue
		}
		if lt := c.declaredLifetime(w.Iface); lt != 0 {
			return lt
		}
	}
	return 0
}

// paramLifetimeOf is prm's own named lifetime as declared on info; unlike
// paramLifetime it never resolves self's elided lifetime.
func (c *borrowCheck) paramLifetimeOf(info *sem.FnInfo, prm sem.EntityID) sem.EntityID {
	if lt := c.p.R.Local(prm).Lifetime; lt != 0 {
		return lt
	}
	if lt := c.witnessBoundLifetime(info, c.p.R.Entity(prm).Type); lt != 0 {
		return lt
	}
	return c.soleLifetime(c.p.R.Entity(prm).Type)
}

// selfLifetime is the current function's own receiver's sole lifetime
// parameter, 0 when there is none or more than one to name unambiguously.
func (c *borrowCheck) selfLifetime() sem.EntityID {
	if c.p.R.Entity(c.f.Ent).Kind != sem.EntFn {
		return 0
	}
	var sole sem.EntityID
	for _, p := range c.recvLifetimeParams(c.p.R.Fn(c.f.Ent)) {
		if !c.p.R.TypeParam(p).IsLifetime {
			continue
		}
		if sole != 0 {
			return 0
		}
		sole = p
	}
	return sole
}

// checkLifetimeEscape rejects returning a param-inherited holder under a lifetime
// it cannot prove matches the function's own declared return lifetime.
func (c *borrowCheck) checkLifetimeEscape(a LocalID, b *borrowInfo) {
	if b.empty || c.closureLifetime(b) && c.retLifetime() != 0 {
		return
	}
	if ret := c.retLifetime(); ret != 0 && b.lifetime == ret {
		return
	}
	node := b.node
	if def := c.definition(a); def != nil {
		node = def.Node
	}
	if b.multi != 0 && !b.hasOwner && !b.fromStatic {
		c.reportCode(node, a, "borrow-multi-lifetime", c.p.R.TypeString(c.stripRef(b.multi)))
		return
	}
	name := "lifetime-untracked-return"
	if b.fromStatic {
		name = "static-borrow-escape"
	}
	code, msg := lifetimeUntrackedDiag(name)
	c.report(node, a, code, msg)
}

// lifetimeUntrackedDiag renders an E1040/E1041 diagnostic from the sem catalog
// instead of a hand-written literal.
func lifetimeUntrackedDiag(name string) (string, string) {
	c, _ := sem.CodeByName(name)
	msg := c.Template
	if c.Help != "" {
		msg += "; " + c.Help
	}
	return c.Num, msg
}

// retLifetime is the current function's own return lifetime: the declared
// one, else the sole lifetime its non-reference return type names inside.
func (c *borrowCheck) retLifetime() sem.EntityID {
	if c.p.R.Entity(c.f.Ent).Kind == sem.EntFn && c.p.R.Fn(c.f.Ent).RetLifetime != 0 {
		return c.p.R.Fn(c.f.Ent).RetLifetime
	}
	if c.p.R.Types.Kind(c.f.Ret) == sem.KRef {
		return 0
	}
	return c.soleLifetime(c.f.Ret)
}

// closureLifetime reports a closure's holder read through a parameter's referent: a closure's
// lifetimes are inference defaults, and its caller ties the results to what it was given.
func (c *borrowCheck) closureLifetime(b *borrowInfo) bool {
	return !b.hasOwner && b.lifetime != 0 && c.p.R.Entity(c.f.Ent).Kind == sem.EntClosure
}
