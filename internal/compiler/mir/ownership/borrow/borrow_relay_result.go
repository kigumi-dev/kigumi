package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// relayedFnItems needs the callee's result to be an unconstrained type parameter of its own whose other type parameters
// cannot produce one, so the body gets one only from a parameter, which must be an argument whose fn items follow knows.
func (c *borrowCheck) relayedFnItems(in *midir.Inst, depth int, follow func(midir.LocalID, int) fnItems) fnItems {
	r := c.p.R
	if in.Ent == 0 || r.Entity(in.Ent).Kind != sem.EntFn {
		return fnItems{}
	}
	info := r.Fn(in.Ent)
	ret := r.Types.Node(info.Sig).Elem
	n := r.Types.Node(ret)
	if n.Kind != sem.KParam || !slices.Contains(info.TypeParams, n.Ent) || len(r.TypeParam(n.Ent).Constraints) > 0 ||
		info.Declared&sem.EffUnsafe != 0 || info.Abi != "" {
		return fnItems{}
	}
	if slices.ContainsFunc(info.TypeParams, func(q sem.EntityID) bool {
		return q != n.Ent && c.boundYields(q, n.Ent, map[sem.EntityID]bool{})
	}) {
		return fnItems{}
	}
	s := c.keptOf(in.Ent)
	out, followed := knownItems(), false
	for k, q := range c.declaredParams(in.Ent) {
		if t := r.Entity(q).Type; t != ret {
			if c.yieldsParam(t, n.Ent, map[sem.EntityID]bool{}) {
				return fnItems{}
			}
			continue
		}
		if k >= len(in.Args) {
			return fnItems{}
		}
		if s != nil && !s.keepsResult(k) {
			continue
		}
		followed, out = true, out.join(follow(in.Args[k], depth))
	}
	if !followed {
		return fnItems{}
	}
	return out
}

// yieldsParam reports whether a value of type t could give the body a value of type parameter p: t names p, or names a
// type parameter whose bounds can produce one.
func (c *borrowCheck) yieldsParam(t sem.TypeID, p sem.EntityID, seen map[sem.EntityID]bool) bool {
	tt := c.p.R.Types
	return tt.Contains(t, tt.Param(p)) || c.makesAny(t) ||
		slices.ContainsFunc(tt.Params(t), func(q sem.EntityID) bool { return c.boundYields(q, p, seen) })
}

// boundYields reports whether the bounds of type parameter q can produce a value of p: an `Fn` result or an interface
// requirement, called on a value of q or on q itself without a receiver.
func (c *borrowCheck) boundYields(q, p sem.EntityID, seen map[sem.EntityID]bool) bool {
	if seen[q] {
		return false
	}
	seen[q] = true
	return slices.ContainsFunc(c.p.R.TypeParam(q).Constraints, func(k sem.Constraint) bool {
		return k.Type != 0 && c.yieldsParam(k.Type, p, seen)
	})
}

// makesAny reports an interface with a requirement whose result names a type parameter of the requirement's own, which
// a caller may instantiate with any type.
func (c *borrowCheck) makesAny(t sem.TypeID) bool {
	r, tt := c.p.R, c.p.R.Types
	if tt.Kind(t) != sem.KIface {
		return false
	}
	return slices.ContainsFunc(r.Iface(tt.Node(t).Ent).Reqs, func(req sem.EntityID) bool {
		res := tt.Node(r.Fn(req).Sig).Elem
		return slices.ContainsFunc(r.Fn(req).TypeParams, func(q sem.EntityID) bool {
			return !r.TypeParam(q).IsLifetime && tt.Contains(res, tt.Param(q))
		})
	})
}
