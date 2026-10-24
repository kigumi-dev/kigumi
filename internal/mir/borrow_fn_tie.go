package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// tiedCallSource adds to what a call's result borrows the reference arguments a passed function value can borrow from.
func (c *borrowCheck) tiedCallSource(in Inst) (*borrowInfo, bool) {
	b, ok := c.calleeSource(in)
	tie := c.fnTie(in)
	if len(tie) == 0 {
		return b, ok
	}
	s := c.sink(in.Dst, in.Node)
	if ok {
		c.merge(s, b)
	}
	for _, t := range tie {
		c.merge(s, t)
	}
	return s, true
}

// fnTie lists the borrows of a call's reference arguments and receiver when it passes a function value whose result
// borrows its own arguments for a parameter returning the result's type parameter: the callee can build its result from them.
func (c *borrowCheck) fnTie(in Inst) []*borrowInfo {
	info := c.p.R.Fn(in.Ent)
	want := c.typeParamsOf(c.p.R.Types.Node(info.Sig).Elem)
	dst := c.f.Locals[in.Dst].Type
	generic := !c.holdsBorrow(dst) && c.namesTypeParam(dst)
	if len(want) == 0 || !generic && !c.holdsBorrow(dst) {
		return nil
	}
	slots := c.callSlots(info, in.Args, 0)
	if !c.passesBorrowMaker(slots, want) {
		return nil
	}
	var out []*borrowInfo
	for i, s := range slots {
		var b *borrowInfo
		ok := false
		switch {
		case i == 0 && info.Recv != sem.RecvNone:
			if info.Recv == sem.RecvMove {
				continue
			}
			b, ok = c.receiverPlaceInfo(s.arg, info.Recv == sem.RecvMut)
		case c.p.R.Types.Kind(s.exact) == sem.KRef:
			b, ok = c.holders[s.arg]
			if generic && (!ok || !b.hasOwner) {
				b, ok = c.receiverPlaceInfo(s.arg, false)
			}
		}
		if ok && generic {
			b = c.carriedBorrows(b)
			ok = b != nil
		}
		if ok {
			out = append(out, b)
		}
	}
	return out
}

// tieValues are the reference arguments, beyond have, a `&mut` target t may come to borrow through a passed function value.
func (c *borrowCheck) tieValues(slots []callSlot, t callSlot, have []flowArg) []flowArg {
	if !c.passesBorrowMaker(slots, t.params) {
		return nil
	}
	var out []flowArg
	for _, s := range slots {
		v := flowArg{arg: s.arg}
		if s.arg != t.arg && !s.target && c.p.R.Types.Kind(s.exact) == sem.KRef && !slices.Contains(have, v) {
			out = append(out, v)
		}
	}
	return out
}

// passesBorrowMaker reports whether a slot whose function type returns one of want gets a function value whose
// result may borrow its own arguments, or a slot naming one of want gets a value that carries such a function.
func (c *borrowCheck) passesBorrowMaker(slots []callSlot, want []sem.EntityID) bool {
	return slices.ContainsFunc(slots, func(s callSlot) bool {
		if ft := c.slotFnType(s.exact); ft != 0 {
			return sharesParam(c.typeParamsOf(c.p.R.Types.Node(ft).Elem), want) && c.makesOwnBorrow(s.arg)
		}
		return sharesParam(s.params, want) && c.carriesBorrowMaker(c.f.Locals[s.arg].Type, map[sem.TypeID]bool{})
	})
}

// carriesBorrowMaker reports whether a value of type t holds, in an argument, element, field or payload, a function
// whose result may borrow its arguments.
func (c *borrowCheck) carriesBorrowMaker(t sem.TypeID, seen map[sem.TypeID]bool) bool {
	if t == 0 || seen[t] {
		return false
	}
	seen[t] = true
	n := c.p.R.Types.Node(t)
	if r := c.fnResult(t); r != 0 {
		return c.mayBorrowResult(r)
	}
	if c.carriesBorrowMaker(n.Elem, seen) || slices.ContainsFunc(n.Args, func(a sem.TypeID) bool { return c.carriesBorrowMaker(a, seen) }) {
		return true
	}
	if n.Kind != sem.KNamed || c.p.R.Entity(n.Ent).Kind != sem.EntType || len(n.Args) != len(c.p.R.TypeDecl(n.Ent).Params) {
		return false
	}
	d := c.p.R.TypeDecl(n.Ent)
	parts := slices.Clone(c.p.R.AggFields(t))
	for _, v := range d.Variants {
		parts = append(parts, c.p.R.AggPayload(t, v)...)
	}
	return slices.ContainsFunc(parts, func(p sem.TypeID) bool { return c.carriesBorrowMaker(p, seen) })
}

// mayBorrowResult reports whether a function returning r, whose definition is not known, may return a borrow.
func (c *borrowCheck) mayBorrowResult(r sem.TypeID) bool {
	return c.holdsBorrow(r) || c.namesTypeParam(r)
}

// makesOwnBorrow reports whether the function value a returns a reference, or names a lifetime of its own declaration;
// a lifetime a closure names belongs to the enclosing function. An unknown value is judged by its type.
func (c *borrowCheck) makesOwnBorrow(a LocalID) bool {
	switch fn := c.fnEntityOf(a); {
	case fn != 0 && c.p.R.Entity(fn).Kind == sem.EntFn:
		info := c.p.R.Fn(fn)
		ret := c.p.R.Types.Node(info.Sig).Elem
		return c.refIn(ret) || slices.ContainsFunc(c.p.R.Types.Params(ret), func(p sem.EntityID) bool {
			return c.p.R.TypeParam(p).IsLifetime && slices.Contains(info.TypeParams, p)
		})
	case fn != 0 && c.p.R.Entity(fn).Kind == sem.EntClosure:
		return c.refIn(c.p.R.Closure(fn).Ret)
	}
	r := c.fnResult(c.f.Locals[a].Type)
	return r != 0 && c.mayBorrowResult(r)
}

// fnEntityOf follows a's single definitions through copies to the function item or closure it names.
func (c *borrowCheck) fnEntityOf(a LocalID) sem.EntityID {
	for range len(c.f.Locals) {
		d := soleDef(c.f, a)
		switch {
		case d == nil:
			return 0
		case d.Op == OpFnItem || d.Op == OpClosure:
			return d.Ent
		case (d.Op == OpCopy || d.Op == OpMove || d.Op == OpAlias) && len(d.Args) > 0:
			a = d.Args[0]
		default:
			return 0
		}
	}
	return 0
}

// refIn reports whether t is or has as an argument a reference, not looking into a named type's fields.
func (c *borrowCheck) refIn(t sem.TypeID) bool {
	n := c.p.R.Types.Node(t)
	return n.Kind == sem.KRef || n.Elem != 0 && c.refIn(n.Elem) || slices.ContainsFunc(n.Args, c.refIn)
}

// slotFnType is the function type a parameter of type t calls: t itself, or the `Fn` bound of a type parameter.
func (c *borrowCheck) slotFnType(t sem.TypeID) sem.TypeID {
	switch c.p.R.Types.Kind(t) {
	case sem.KFn:
		return t
	case sem.KParam:
		for _, k := range c.p.R.TypeParam(c.p.R.Types.Node(t).Ent).Constraints {
			if (k.Kind == sem.CFn || k.Kind == sem.CFnMut || k.Kind == sem.CFnOnce) && c.p.R.Types.Kind(k.Type) == sem.KFn {
				return k.Type
			}
		}
	}
	return 0
}

func (c *borrowCheck) fnResult(t sem.TypeID) sem.TypeID {
	n := c.p.R.Types.Node(t)
	switch {
	case n.Kind == sem.KFn:
		return n.Elem
	case n.Kind == sem.KClosure && n.Ent != 0 && c.p.R.Entity(n.Ent).Kind == sem.EntClosure:
		return c.p.R.Closure(n.Ent).Ret
	}
	return 0
}

func (c *borrowCheck) holdsBorrow(t sem.TypeID) bool {
	return c.typeHolds(t, func(t sem.TypeID) bool {
		return c.p.R.Types.Kind(t) == sem.KRef || c.declaredLifetime(t) != 0
	})
}
