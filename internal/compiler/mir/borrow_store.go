package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type storeFlow struct {
	target     midir.LocalID
	values     []flowArg
	ref        bool
	unresolved bool
	// generic marks a flow found only through a type parameter the target's type names.
	generic bool
	// callee and param name the parameter a call's `&mut` argument is lent to; callee is 0 for a plain store.
	callee sem.EntityID
	param  int
	// opaque marks a `&mut` argument of a call through a function value, whose body is unknown.
	opaque bool
	// kept marks a flow a keptSummary or the unknown-callee default finds; like generic, it stores this frame's borrows.
	kept bool
	// byValue marks a holder handed over by value: what is stored lands in the places its `&mut`s reach.
	byValue bool
	// args are a named call's arguments by parameter, for the callee's sites that hand on to code an argument gives.
	args []midir.LocalID
}

// storeTargets finds, before any holder is known, the locals some store may
// write a borrow-carrying value into, so collect keeps their sources widening.
func (c *borrowCheck) storeTargets() map[midir.LocalID]bool {
	out := map[midir.LocalID]bool{}
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			for _, fl := range c.storeFlows(in) {
				if root, _, caller, ok := c.storeRoot(fl); ok && !caller && !fl.byValue {
					out[root] = true
				}
			}
		}
	}
	return out
}

func (c *borrowCheck) storeFlows(in midir.Inst) []storeFlow {
	switch in.Op {
	case midir.OpSetIndex, midir.OpSetField:
		v := in.Args[len(in.Args)-1]
		lifetime := c.elementHolder(c.f.Locals[v].Type) || c.borrowIntoHolderField(in, v)
		if !lifetime && !c.namesTypeParam(c.f.Locals[v].Type) {
			return nil
		}
		return []storeFlow{{target: in.Args[0], values: []flowArg{{arg: v}}, generic: !lifetime}}
	case midir.OpCall, midir.OpCallValue, midir.OpFutureNew:
		return c.withKeptFlows(in, append(c.callFlows(in), c.opaqueFlows(in)...))
	}
	return nil
}

// borrowIntoHolderField reports a plain borrow (not an `&Interface` view, which lives in its frame) assigned to a field of a record that names a lifetime.
func (c *borrowCheck) borrowIntoHolderField(in midir.Inst, v midir.LocalID) bool {
	tt := c.p.R.Types
	if in.Op != midir.OpSetField || tt.Kind(c.f.Locals[v].Type) != sem.KRef {
		return false
	}
	if tt.Kind(tt.Node(c.f.Locals[v].Type).Elem) == sem.KIface {
		return false
	}
	rec := c.f.Locals[in.Args[0]].Type
	if tt.Kind(rec) == sem.KRef {
		rec = tt.Node(rec).Elem
	}
	return c.declaredLifetime(rec) != 0
}

// storeRoot resolves where a flow writes: the named local owning the place, the place's type, and whether it is the
// caller's: a lent parameter, or the parameter whose `&mut`s the holder written through may keep.
func (c *borrowCheck) storeRoot(fl storeFlow) (root midir.LocalID, place sem.TypeID, caller, ok bool) {
	if fl.unresolved {
		return 0, 0, false, false
	}
	l := fl.target
	place = c.f.Locals[l].Type
	if fl.byValue {
		l = c.valueSource(l)
		if p, ok := c.heldOrigin(l); ok {
			return p, c.f.Locals[l].Type, true, true
		}
		return l, c.f.Locals[l].Type, false, true
	}
	if fl.ref {
		for c.f.Locals[l].Ent == 0 {
			in := c.definition(l)
			if in == nil || len(in.Args) == 0 {
				return 0, 0, false, false
			}
			if in.Op == midir.OpBorrow {
				l = in.Args[0]
				place = c.f.Locals[l].Type
				break
			}
			if in.Op != midir.OpNewCell && in.Op != midir.OpShare && in.Op != midir.OpCopy && in.Op != midir.OpMove && in.Op != midir.OpAlias {
				return 0, 0, false, false
			}
			l = in.Args[0]
		}
	}
	if c.p.R.Types.Kind(place) == sem.KRef {
		place = c.p.R.Types.Node(place).Elem
	}
	if p, ok := c.heldRead(l); ok {
		return p, place, true, true
	}
	if refs := c.referents(l); len(refs) > 0 {
		l = refs[0].owner
	}
	root, _, _, ok = c.projectionPlace(l)
	if ok && c.f.Locals[root].Ent != 0 && !slices.Contains(c.f.Params, root) && c.p.R.Types.Kind(c.f.Locals[root].Type) == sem.KRef {
		root, ok = c.refRoot(root)
	}
	if !ok {
		return 0, 0, false, false
	}
	if c.callerPlace(root) {
		return root, place, true, true
	}
	if c.p.R.Types.Kind(c.f.Locals[root].Type) == sem.KRef {
		return 0, 0, false, false
	}
	return root, place, false, true
}

// callerPlace reports whether l is a parameter aliasing the caller's value:
// a reference, or a `self` / `mut self` receiver.
func (c *borrowCheck) callerPlace(l midir.LocalID) bool {
	if !slices.Contains(c.f.Params, l) {
		return false
	}
	return c.p.R.Types.Kind(c.f.Locals[l].Type) == sem.KRef || c.f.Locals[l].Borrowed
}

// checkFlow rejects a write into a place whose type keeps no sources, or into the caller's
// place unless the value carries this function's own lifetime that the place names.
func (c *borrowCheck) checkFlow(fl storeFlow, in midir.Inst) {
	c.callNode = in.Node
	root, place, caller, ok := c.storeRoot(fl)
	for _, v := range fl.values {
		b, has := c.storedInfo(fl, v, caller)
		if !has || b.empty || v.at != nil {
			continue
		}
		switch {
		case !ok:
			c.reportCode(in.Node, v.arg, "borrow-store-untracked", c.untrackedPlace(fl))
		case !c.placeKeeps(place, v) && (v.tie == tieNone || !c.callerKept(b)) && (!fl.byValue || c.closureLiteral(root) == nil):
			c.reportCode(in.Node, v.arg, "borrow-store-untracked", c.typeName(place))
		case !caller && c.keepsTemp(b):
			c.reportCode(in.Node, v.arg, "temporary-borrow-kept", c.f.Locals[root].Name, tempDropped)
			c.noteKept(b)
		case !caller && c.borrowsFrom(b, root):
			name := c.f.Locals[root].Name
			c.report(in.Node, v.arg, "borrow-escape", "a borrow of `"+name+"` cannot be stored back into `"+name+"`")
		case caller && !c.fitsOwnLifetime(b, place):
			c.reportCode(in.Node, v.arg, "borrow-store-outlives", c.f.Locals[root].Name)
		}
	}
	if ok && !caller {
		c.checkHeldCallers(fl, in)
	}
}

func (c *borrowCheck) fitsOwnLifetime(b *borrowInfo, place sem.TypeID) bool {
	if b.hasOwner {
		return false
	}
	if b.fromStatic {
		return true
	}
	if b.lifetime == 0 || b.lifetime != c.soleLifetime(place) || c.p.R.Entity(c.f.Ent).Kind != sem.EntFn {
		return false
	}
	info := c.p.R.Fn(c.f.Ent)
	return slices.Contains(info.TypeParams, b.lifetime) || slices.Contains(c.recvLifetimeParams(info), b.lifetime)
}
