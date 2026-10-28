package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// tiedCallSource adds to what a call's result borrows the reference arguments a passed function value can borrow from.
func (c *borrowCheck) tiedCallSource(in Inst) (*borrowInfo, bool) {
	b, ok := c.calleeSource(in)
	tie, done := c.fnTie(in)
	tie = append(tie, c.summaryTie(in, done)...)
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

// fnTie: a passed fn value whose result may borrow its arguments lets the callee build its result from them; args are
// the arguments it judged.
func (c *borrowCheck) fnTie(in Inst) (out []*borrowInfo, args []LocalID) {
	info := c.p.R.Fn(in.Ent)
	want := c.wantOf(c.p.R.Types.Node(info.Sig).Elem)
	dst := c.f.Locals[in.Dst].Type
	generic := !c.holdsBorrow(dst) && c.namesTypeParam(dst)
	if want.empty() || !generic && !c.tieCarrier(dst) {
		return nil, nil
	}
	for _, v := range c.tieArgs(info, c.callSlots(info, in.Args, 0), want) {
		args = append(args, v.arg)
		b, ok := c.tieInfo(v, generic, false)
		if !ok && v.tie == tieRef {
			b, ok = c.lentCellBorrow(v.arg, in.Dst)
		}
		if ok && generic {
			b = c.carriedBorrows(b)
			ok = b != nil
		}
		if ok {
			out = append(out, b)
		}
	}
	return out, args
}

func (c *borrowCheck) tieValues(info *sem.FnInfo, slots []callSlot, t callSlot, have []flowArg) []flowArg {
	var out []flowArg
	decl := t.exact
	if decl == 0 && info.SelfParam != 0 {
		decl = c.p.R.Entity(info.SelfParam).Type
	}
	want := c.wantOf(decl)
	want.params = t.params
	for _, v := range c.tieArgs(info, slots, want) {
		if v.arg != t.arg && !slices.ContainsFunc(have, func(h flowArg) bool { return h.arg == v.arg }) {
			out = append(out, v)
		}
	}
	return out
}

// carriesBorrowMaker reports whether a value of type t holds, in an argument, element, field or payload, a function
// whose result may borrow its arguments.
func (c *borrowCheck) carriesBorrowMaker(t sem.TypeID, seen map[sem.TypeID]bool) bool {
	return c.carriesMaker(t, seen, c.mayBorrowResult)
}

// carriesMaker: t holds a function whose result, or the result of a function that result holds, satisfies yields.
func (c *borrowCheck) carriesMaker(t sem.TypeID, seen map[sem.TypeID]bool, yields func(sem.TypeID) bool) bool {
	if t == 0 || seen[t] {
		return false
	}
	seen[t] = true
	n := c.p.R.Types.Node(t)
	if r := c.fnResult(t); r != 0 {
		return yields(r) || c.carriesMaker(r, seen, yields)
	}
	in := func(a sem.TypeID) bool { return c.carriesMaker(a, seen, yields) }
	if in(n.Elem) || slices.ContainsFunc(n.Args, in) {
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
	return slices.ContainsFunc(parts, in)
}

// mayBorrowResult reports whether a function returning r, whose definition is not known, may return a borrow.
func (c *borrowCheck) mayBorrowResult(r sem.TypeID) bool {
	return c.holdsBorrow(r) || c.namesTypeParam(r)
}

// makesOwnBorrow reports whether the function value a returns a reference, names a lifetime of its own declaration (a
// closure's belongs to the enclosing function) or keeps an argument's Future or capture. An unknown value is judged by its type.
func (c *borrowCheck) makesOwnBorrow(a LocalID) bool {
	switch fn := c.fnEntityOf(a); {
	case fn != 0 && c.p.isAsyncBody(fn):
		return c.asyncKeepsBorrow(c.p.R.Fn(fn))
	case fn != 0 && c.p.R.Entity(fn).Kind == sem.EntFn:
		info := c.p.R.Fn(fn)
		ret := c.p.R.Types.Node(info.Sig).Elem
		return c.refIn(ret) || c.keepsDeferredArg(info.Params, ret) || slices.ContainsFunc(c.p.R.Types.Params(ret), func(p sem.EntityID) bool {
			return c.p.R.TypeParam(p).IsLifetime && slices.Contains(info.TypeParams, p)
		})
	case fn != 0 && c.p.R.Entity(fn).Kind == sem.EntClosure:
		info := c.p.R.Closure(fn)
		return c.refIn(info.Ret) || c.keepsDeferredArg(info.Params, info.Ret)
	}
	r := c.fnResult(c.f.Locals[a].Type)
	return r != 0 && c.mayBorrowResult(r)
}

// fnEntityOf follows a's single definitions through copies to the function item or closure it names.
func (c *borrowCheck) fnEntityOf(a LocalID) sem.EntityID {
	for range len(c.f.Locals) {
		d := c.soleDef(a)
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
