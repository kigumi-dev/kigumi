package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// implBounds holds a value of named type val, met as interface iface, to the Fn bounds put on val's parameters as val's
// arguments instantiate them; an erasure is refused when iface's methods take values unproved.
func (w *slotWalk) implBounds(val, iface TypeID, at erasePos) {
	tt := w.c.r.Types
	n := tt.Node(val)
	if n.Kind != KNamed || w.c.r.Entities[n.Ent].Kind != EntType || tt.Kind(iface) != KIface {
		return
	}
	sub := &slotWalk{c: w.c, seen: w.seen}
	for _, binders := range w.c.boundBinders(val, iface) {
		params := make([]TypeID, len(binders))
		for i, p := range binders {
			params[i] = tt.Param(p)
		}
		sub.subst = paramSubst(binders, n.Args)
		sub.walk(tt.Named(n.Ent, params), at)
	}
	w.refuse = w.refuse || sub.refuse || len(sub.slots) > 0 && w.c.ifaceTakesValues(iface)
	w.slots = append(w.slots, sub.slots...)
}

// boundBinders are the lists binding val's parameters with bounds where val meets iface: its declaration's, and the
// receiver binders of the methods conformance picks to answer iface's requirements.
func (c *checker) boundBinders(val, iface TypeID) [][]EntityID {
	decl := c.r.typeDecl(c.r.Types.Node(val).Ent).Params
	out := [][]EntityID{decl}
	for _, m := range c.r.conforms(val, iface, c.pkg).witnesses {
		if rp := c.r.Fn(m).RecvParams; len(rp) > 0 && len(rp) == len(decl) {
			out = append(out, rp)
		}
	}
	return out
}

// ifaceTakesValues reports whether a method of iface, its type arguments substituted, has a parameter that can hold a
// callable: a fn type anywhere in it, or a type parameter the method declares, which the implementer may bound.
func (c *checker) ifaceTakesValues(iface TypeID) bool {
	tt := c.r.Types
	n := tt.Node(c.vars.resolve(iface))
	info := c.r.iface(n.Ent)
	subst := map[EntityID]TypeID{}
	for i, p := range info.Params {
		// Lifetime arguments stay parameters: they cannot make a type hold a callable, and an open one reads as any type.
		if i < len(n.Args) && !c.r.typeParam(p).IsLifetime {
			subst[p] = n.Args[i]
		}
	}
	for _, req := range info.Reqs {
		fn := c.r.Fn(req)
		for _, a := range tt.Node(tt.Subst(fn.Sig, subst)).Args {
			if c.r.mayContainFn(a, map[TypeID]bool{}) || slices.ContainsFunc(fn.TypeParams, func(p EntityID) bool {
				return !c.r.typeParam(p).IsLifetime && tt.Contains(a, tt.Param(p))
			}) {
				return true
			}
		}
	}
	return false
}

// implContracts holds a value at n of type got, made an interface value want, to got's own Fn bounds.
func (c *checker) implContracts(n syntax.NodeID, got, want TypeID, at erasePos) {
	w := &slotWalk{c: c, seen: map[EntityID]bool{}}
	w.implBounds(c.vars.resolve(got), c.vars.resolve(want), at)
	c.recordWalk(n, w)
}
