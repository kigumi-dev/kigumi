package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// holdsMutPlace reports a value, not a reference, whose type keeps a `&mut` of some place.
func (c *borrowCheck) holdsMutPlace(l LocalID) bool { return c.mutHolder(c.f.Locals[l].Type) }

// mayHoldPlace reports a local a of g that keeps a `&mut`, or whose layout a type parameter or interface hides.
func (c *borrowCheck) mayHoldPlace(g *Func, a LocalID) bool {
	return c.mutHolder(g.Locals[a].Type) || c.hidesLayout(g.Locals[a].Type)
}

// hidesLayout reports a value of t, not behind a reference, part of which is a type parameter, interface or
// unresolved type.
func (c *borrowCheck) hidesLayout(t sem.TypeID) bool {
	tt := c.p.R.Types
	return c.typeHoldsUntil(t, func(u sem.TypeID) bool {
		switch n := tt.Node(u); n.Kind {
		case sem.KIface, sem.KVar:
			return true
		case sem.KParam:
			return !c.p.R.TypeParam(n.Ent).IsLifetime
		}
		return false
	}, func(u sem.TypeID) bool { k := tt.Kind(u); return k == sem.KFn || k == sem.KClosure || k == sem.KRef })
}

func (c *borrowCheck) mutHolder(t sem.TypeID) bool {
	return c.p.R.Types.Kind(t) != sem.KRef && c.keepsMutRef(t)
}

func (c *borrowCheck) keepsMutRef(t sem.TypeID) bool {
	held, ok := c.mutHolders[t]
	if !ok {
		held = c.storesMutRef(t)
		if c.mutHolders == nil {
			c.mutHolders = map[sem.TypeID]bool{}
		}
		c.mutHolders[t] = held
	}
	return held
}

// linkedParams are the parameters slot i shares with what code may run it with: those its declared type's bounds name,
// and those of each other callable sharing one that may store through what it is called with.
func (c *borrowCheck) linkedParams(slots []callSlot, i int) []sem.EntityID {
	out := withParams(slots[i].params, c.typeBoundParams(slots[i].exact))
	for grown := true; grown; {
		grown = false
		for j, s := range slots {
			ps := withParams(s.params, c.typeBoundParams(s.exact))
			if j == i || !c.callableArg(s.arg) || !sharesParam(out, ps) || !c.callableMayStore(s.arg) {
				continue
			}
			if next := withParams(out, ps); len(next) > len(out) {
				out, grown = next, true
			}
		}
	}
	return out
}

// typeBoundParams are the parameters named by the bounds of the type parameters t names.
func (c *borrowCheck) typeBoundParams(t sem.TypeID) []sem.EntityID {
	var out []sem.EntityID
	if t == 0 {
		return nil
	}
	for _, p := range c.p.R.Types.Params(t) {
		for _, con := range c.p.R.TypeParam(p).Constraints {
			if con.Type != 0 {
				out = withParams(out, c.p.R.Types.Params(con.Type))
			}
		}
	}
	return out
}

func withParams(a, b []sem.EntityID) []sem.EntityID {
	out := a
	for _, p := range b {
		if !slices.Contains(out, p) {
			out = append(slices.Clip(out), p)
		}
	}
	return out
}

// selfSources are the paths of the argument in slot that the callee may store back under the same argument; nil for none.
func (c *borrowCheck) selfSources(callee sem.EntityID, slot int, slots []callSlot) []stepPath {
	if g := c.p.ByEnt[callee]; g != nil {
		return c.paramSelfSources(g, slot)
	}
	if c.seesThrough(callee, slot) || slices.ContainsFunc(slots, func(s callSlot) bool {
		return s.arg != slots[slot].arg && c.callableArg(s.arg) && c.callableMayStore(s.arg)
	}) {
		return []stepPath{{}}
	}
	return nil
}

// callableArg reports an argument that may be code the callee runs: a fn, closure or interface value, or one inside it.
func (c *borrowCheck) callableArg(l LocalID) bool {
	t := c.f.Locals[l].Type
	v, ok := c.callableTypes[t]
	if !ok {
		v = c.typeHolds(t, c.callableNode)
		if c.callableTypes == nil {
			c.callableTypes = map[sem.TypeID]bool{}
		}
		c.callableTypes[t] = v
	}
	return v
}

// partInfo is what v's argument holds at v.at, with what its `&mut`s there lead to; the whole of it when unknown.
func (c *borrowCheck) partInfo(v flowArg) (*borrowInfo, bool) {
	whole := v
	whole.at = nil
	base, ok := c.valueSource(v.arg), true
	if v.contents {
		base, ok = c.flowPlace(storeFlow{target: v.arg, ref: true})
	}
	h, held := c.holders[base]
	if !ok || !held || !h.hasOwner {
		return c.flowInfo(whole)
	}
	out := &borrowInfo{empty: true, node: h.node}
	for _, p := range v.at {
		if owners := c.heldAt(ownerRef{owner: base, exact: true}, h.owners, readHop{path: p, exact: true}); len(owners) > 0 {
			mergeInto(out, &borrowInfo{owners: owners, hasOwner: true, node: h.node})
		}
	}
	if out.empty {
		return nil, false
	}
	return c.withHeld(out), true
}

func slotArgs(slots []callSlot) []LocalID {
	out := make([]LocalID, len(slots))
	for i, s := range slots {
		out[i] = s.arg
	}
	return out
}
