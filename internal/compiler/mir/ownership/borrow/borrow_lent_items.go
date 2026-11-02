package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// lentWalk follows a path of the place lent as argument arg of call in over decl, the type its callee declares for it.
type lentWalk struct {
	c     *borrowCheck
	in    *midir.Inst
	arg   int
	decl  sem.TypeID
	depth int
}

// lentItems are the fn items call in can leave at path q of the place of type t lent as its argument arg, read off the
// callee's signature: a part typed by its own type parameter gets only what the arguments declared with it hold.
func (c *borrowCheck) lentItems(in *midir.Inst, arg int, t sem.TypeID, q []sem.PartStep, depth int) fnItems {
	r := c.p.R
	e := in.Ent
	if e == 0 || r.Entity(e).Kind != sem.EntFn || ownquery.IsAsyncBody(r, e) || c.ifaceRequirement(e) {
		return fnItems{}
	}
	info := r.Fn(e)
	params := c.declaredParams(e)
	if info.Declared&sem.EffUnsafe != 0 || info.Abi != "" || arg >= len(params) || len(in.Args) < len(params) {
		return fnItems{}
	}
	decl, mut := r.MutRefElem(r.Entity(params[arg]).Type)
	if !mut {
		if params[arg] != info.SelfParam || info.Recv != sem.RecvMut || r.Types.Kind(r.Entity(params[arg]).Type) == sem.KRef {
			return fnItems{}
		}
		decl = r.Entity(params[arg]).Type
	}
	w := lentWalk{c: c, in: in, arg: arg, decl: decl, depth: depth}
	return w.at(decl, t, nil, q)
}

// at walks q over d and the place's own type t side by side, so a type argument step names the same argument in both.
func (w lentWalk) at(d, t sem.TypeID, prefix, q []sem.PartStep) fnItems {
	tt := w.c.p.R.Types
	n, pn := tt.Node(d), tt.Node(t)
	switch {
	case n.Kind == sem.KParam && w.own(n.Ent):
		return w.param(n.Ent, prefix, q, prefix)
	case len(q) == 0:
		return w.whole(d, prefix)
	case n.Kind == sem.KNamed && pn.Kind == sem.KNamed && n.Ent == pn.Ent && !q[0].Field && q[0].Index < min(len(n.Args), len(pn.Args)):
		return w.at(n.Args[q[0].Index], pn.Args[q[0].Index], slices.Concat(prefix, q[:1]), q[1:])
	}
	return fnItems{}
}

// whole is everything the callee can leave anywhere in the part of type d at prefix: only its own type parameters may
// hold a callable there.
func (w lentWalk) whole(d sem.TypeID, prefix []sem.PartStep) fnItems {
	tt := w.c.p.R.Types
	own := slices.DeleteFunc(tt.Params(d), func(p sem.EntityID) bool { return !w.own(p) })
	inert := map[sem.EntityID]sem.TypeID{}
	for _, p := range own {
		inert[p] = sem.TyBool
	}
	if !w.c.holdsNoCallable(tt.Subst(d, inert), map[sem.TypeID]bool{}) {
		return fnItems{}
	}
	out := knownItems()
	for _, p := range own {
		paths, exact := w.c.paramPaths(d, tt.Param(p))
		if !exact {
			return fnItems{}
		}
		for _, path := range paths {
			if out = out.join(w.param(p, slices.Concat(prefix, path), nil, prefix)); !out.known {
				return out
			}
		}
	}
	return out
}

// param are the fn items at rest within the values of p the callee can leave at prefix: what the arguments declared with
// p hold. The values of p already in the place must lie under cover, which the proof reads in full.
func (w lentWalk) param(p sem.EntityID, prefix, rest, cover []sem.PartStep) fnItems {
	c, r, tt := w.c, w.c.p.R, w.c.p.R.Types
	if c.ownBoundYields(p) || slices.ContainsFunc(r.Fn(w.in.Ent).TypeParams, func(q sem.EntityID) bool {
		return q != p && c.boundYields(q, p, map[sem.EntityID]bool{})
	}) {
		return fnItems{}
	}
	inPlace, exact := c.paramPaths(w.decl, tt.Param(p))
	if !exact || slices.ContainsFunc(inPlace, func(path []sem.PartStep) bool { return !hasPrefix(path, cover) }) {
		return fnItems{}
	}
	out := knownItems()
	for k, q := range c.declaredParams(w.in.Ent) {
		t := r.Entity(q).Type
		if k == w.arg || !c.yieldsParam(t, p, map[sem.EntityID]bool{}) {
			continue
		}
		paths, exact := c.paramPaths(t, tt.Param(p))
		if !exact || c.makesAny(t) || len(paths) == 0 {
			return fnItems{}
		}
		for _, path := range paths {
			if out = out.join(c.fnItemPath(w.in.Args[k], slices.Concat(path, rest), w.depth)); !out.known {
				return out
			}
		}
	}
	return out
}

// lendReturns reports whether call in can hand the place lent as its argument arg back to the caller, through its result
// or another argument's place: as the callee's kept summary says, or for one without, whenever those can hold a borrow.
func (c *borrowCheck) lendReturns(in *midir.Inst, arg int) bool {
	if params := c.declaredParams(in.Ent); c.p.R.Entity(in.Ent).Kind == sem.EntFn && arg < len(params) {
		if s := c.keptOf(in.Ent); s != nil {
			for t := range params {
				if t != arg && s.storesInto(t, arg) {
					return true
				}
			}
			return s.keepsResult(arg)
		}
	}
	if in.Op.Yields() && c.mayHoldAny(c.f.Locals[in.Dst].Type) {
		return true
	}
	for k, a := range in.Args {
		t := c.f.Locals[a].Type
		if elem, mut := c.p.R.MutRefElem(t); mut {
			t = elem
		}
		if k != arg && c.keepsThrough(c.f, a) && c.mayHoldAny(t) {
			return true
		}
	}
	return false
}

func (c *borrowCheck) mayHoldAny(t sem.TypeID) bool { return c.mayHoldBorrow(t) || c.holdsDeferred(t) }

func (w lentWalk) own(p sem.EntityID) bool {
	return slices.Contains(w.c.p.R.Fn(w.in.Ent).TypeParams, p) && !w.c.p.R.TypeParam(p).IsLifetime
}

// ownBoundYields reports whether a bound of p could give a body a value of p: an interface, whose requirements may
// return Self, or an Fn bound whose signature can yield one. Copy, Send and Sync cannot.
func (c *borrowCheck) ownBoundYields(p sem.EntityID) bool {
	return slices.ContainsFunc(c.p.R.TypeParam(p).Constraints, func(k sem.Constraint) bool {
		return k.Kind == sem.CIface || k.Type != 0 && c.yieldsParam(k.Type, p, map[sem.EntityID]bool{p: true})
	})
}

func hasPrefix(path, prefix []sem.PartStep) bool {
	return len(path) >= len(prefix) && slices.Equal(path[:len(prefix)], prefix)
}
