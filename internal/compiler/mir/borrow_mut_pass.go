package mir

import "kigumi/internal/compiler/sem"

// passesMutRef reports whether in hands owner, a `&mut` reference, to a callee's `&mut` parameter, which may replace
// the referent the way an assignment through owner does.
func (c *borrowCheck) passesMutRef(in Inst, owner LocalID) bool {
	tt := c.p.R.Types
	if in.Op != OpCall && in.Op != OpCallValue || tt.Kind(c.f.Locals[owner].Type) != sem.KRef || !tt.RefMut(c.f.Locals[owner].Type) {
		return false
	}
	if in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn {
		recvIdx := 0
		if in.Op == OpCallValue {
			recvIdx = 1
		}
		for _, s := range c.callSlots(c.p.R.Fn(in.Ent), in.Args, recvIdx) {
			if s.target && c.refOf(s.arg) == owner {
				return true
			}
		}
		return false
	}
	if in.Ent != 0 {
		return false
	}
	for i := 1; i < len(in.Args); i++ {
		if c.refOf(in.Args[i]) != owner {
			continue
		}
		t, known := c.callParam(c.f, in, i)
		if !known || tt.Kind(t) == sem.KRef && tt.RefMut(t) {
			return true
		}
	}
	return false
}

// calleeParams are the parameter types of a called function value: a function type, a closure, or a type parameter
// with an `Fn` bound; known is false when the value's type does not say.
func (c *borrowCheck) calleeParams(t sem.TypeID) (params []sem.TypeID, known bool) {
	n := c.p.R.Types.Node(t)
	switch {
	case n.Kind == sem.KClosure && n.Ent != 0 && c.p.R.Entity(n.Ent).Kind == sem.EntClosure:
		for _, prm := range c.p.R.Closure(n.Ent).Params {
			params = append(params, c.p.R.Entity(prm).Type)
		}
		return params, true
	case c.slotFnType(t) != 0:
		return c.p.R.Types.Node(c.slotFnType(t)).Args, true
	}
	return nil, false
}

// refOf follows a reference argument's single definitions through copies to the local it reborrows.
func (c *borrowCheck) refOf(l LocalID) LocalID {
	for range len(c.f.Locals) {
		d := c.soleDef(l)
		if d == nil || len(d.Args) == 0 || d.Op != OpCopy && d.Op != OpMove && d.Op != OpAlias && d.Op != OpShare && d.Op != OpNewCell {
			return l
		}
		l = d.Args[0]
	}
	return l
}
