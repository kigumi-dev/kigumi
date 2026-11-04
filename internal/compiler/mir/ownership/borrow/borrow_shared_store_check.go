package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// checkSharedStores rejects a call storing into shared storage a value that borrows a place this frame owns; a borrow a
// parameter brought in is left to the caller, which sees it through this body's sharedSinks.
func (c *borrowCheck) checkSharedStores(in midir.Inst) {
	x := &sinkCtx{g: c.f}
	var ts []sinkTarget
	if in.Op == midir.OpCall || in.Op == midir.OpCallValue || in.Op == midir.OpFutureNew {
		ts, _ = c.sinkCallees(x, in)
	}
	gate := false
	for k, a := range in.Args {
		keeps := slices.ContainsFunc(ts, func(t sinkTarget) bool { return slices.ContainsFunc(t.bind(k), c.keepsRef) })
		gate = gate || c.ownedBorrow(a, keeps) != nil
	}
	if !gate {
		return
	}
	x.fs = c.bodyFacts(c.f)
	for _, st := range c.sharedStoreArgs(x, in) {
		a := in.Args[st.arg]
		if b := c.flowBorrow(a, st.param != 0 && c.keepsRef(st.param)); b != nil {
			c.reportShared(in, a, b)
			return
		}
	}
}

// keepsRef reports a parameter that may keep the reference it is given, as one whose type names a lifetime may.
func (c *borrowCheck) keepsRef(q sem.EntityID) bool {
	return c.p.R.Entity(q).Kind == sem.EntParam && c.facts().DeclaredLifetime(c.p.R.Entity(q).Type) != 0
}

// ownedBorrow is a borrow of a place this frame owns that a's value carries, nil for none.
func (c *borrowCheck) ownedBorrow(a midir.LocalID, keepsRef bool) *borrowInfo {
	for _, l := range c.carriedLocals(a, keepsRef) {
		if b := c.holders[l]; b.hasOwner && !b.empty {
			return b
		}
	}
	return nil
}

// flowBorrow is ownedBorrow narrowed to the owners the value flow into each carried local confirms (flowOwners).
func (c *borrowCheck) flowBorrow(a midir.LocalID, keepsRef bool) *borrowInfo {
	b := c.ownedBorrow(a, keepsRef)
	if b == nil {
		return nil
	}
	var owners []ownerRef
	seen := flowMemo{}
	for _, l := range c.carriedLocals(a, keepsRef) {
		if h := c.holders[l]; h.hasOwner && !h.empty {
			owners = append(owners, c.flowOwners(l, seen)...)
		}
	}
	if len(owners) == 0 {
		return nil
	}
	return &borrowInfo{owners: owners, hasOwner: true, node: b.node}
}

// carriedLocals are the holders whose borrows a's value carries: a itself, or for a reference the places it points at,
// and the reference too when the parameter it binds names a lifetime, so the callee may keep it.
func (c *borrowCheck) carriedLocals(a midir.LocalID, keepsRef bool) []midir.LocalID {
	return c.carriedFrom(a, keepsRef, map[midir.LocalID]bool{})
}

func (c *borrowCheck) carriedFrom(a midir.LocalID, keepsRef bool, seen map[midir.LocalID]bool) []midir.LocalID {
	if seen[a] {
		return nil
	}
	seen[a] = true
	if d := c.definition(a); c.f.Locals[a].Cell && d != nil && d.Op == midir.OpNewCell && len(d.Args) > 0 {
		return c.carriedFrom(d.Args[0], keepsRef, seen)
	}
	b, ok := c.holders[a]
	if !ok {
		return nil
	}
	if c.p.R.Types.Kind(c.f.Locals[a].Type) != sem.KRef {
		return []midir.LocalID{a}
	}
	var out []midir.LocalID
	if keepsRef {
		out = append(out, a)
	}
	for _, o := range b.owners {
		switch _, ok := c.holders[o.owner]; {
		case seen[o.owner] || !ok:
		case o.view || c.p.R.Types.Kind(c.f.Locals[o.owner].Type) == sem.KRef:
			out = append(out, c.carriedFrom(o.owner, false, seen)...)
		default:
			out = append(out, o.owner)
		}
	}
	return out
}

func (c *borrowCheck) reportShared(in midir.Inst, a midir.LocalID, b *borrowInfo) {
	code, _ := sem.CodeByName("borrow-store-shared")
	name := c.carriedName(b)
	msg, help := sem.FillCode(code, name)
	c.reportHelped(in.Node, a, code.Num, msg, b.node, name+" is borrowed here", help)
}

// carriedName names what b borrows for a diagnostic: its first owner, through an `&Interface` view to the viewed place.
func (c *borrowCheck) carriedName(b *borrowInfo) string {
	for _, o := range b.owners {
		l := o.owner
		if o.view {
			l = c.viewSources(l).locals[0]
			if r, ok := c.refRoot(l); ok {
				l = r
			}
		}
		if name := c.f.Locals[l].Name; c.f.Locals[l].Ent != 0 && name != "" {
			return "`" + name + "`"
		}
	}
	return "a temporary"
}
