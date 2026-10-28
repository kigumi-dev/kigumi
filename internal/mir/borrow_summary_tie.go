package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// summaryTie ties a direct call's Future or Task result to each reference or lent receiver argument, besides those
// fnTie judged, whose parameter the callee's keptSummary shows its result keeping.
func (c *borrowCheck) summaryTie(in Inst, done []LocalID) []*borrowInfo {
	info := c.p.R.Fn(in.Ent)
	ret := c.p.R.Types.Node(info.Sig).Elem
	if c.p.isAsyncBody(in.Ent) || c.fnRetLifetime(info) != 0 || c.resultMayCarryLifetime(ret) || !c.holdsAsync(c.f.Locals[in.Dst].Type) {
		return nil
	}
	s := c.keptBound(in.Ent, c.siteBind(in.Ent, in.Args))
	if s == nil {
		return nil
	}
	params := c.declaredParams(in.Ent)
	var out []*borrowInfo
	for k, a := range in.Args {
		if k >= len(params) || slices.Contains(done, a) || !c.lentByCaller(info, params[k]) || !s.keepsResult(k) {
			continue
		}
		if b, ok := c.keptArgBorrow(in.Dst, a); ok {
			if b = c.carriedBorrows(b); b != nil {
				out = append(out, b)
			}
		}
	}
	return out
}

func (c *borrowCheck) unsummarized(e sem.EntityID) bool {
	return c.p.R.Entity(e).Kind != sem.EntFn || c.keptStoreSummary(e, nil) == nil
}

// asyncPlace reports whether the place a `&mut` argument a lends, of a type no lifetime or type parameter names (those
// callFlows follows), can come to hold a Future or Task: in its type, or behind an interface whose layout is unknown.
func (c *borrowCheck) asyncPlace(a LocalID) bool {
	if c.targetKindOf(a) != targetDeferred {
		return false
	}
	t := c.placeOf(a)
	if t == 0 {
		t = c.f.Locals[a].Type
	}
	return c.asyncType(t)
}

func (c *borrowCheck) asyncType(t sem.TypeID) bool {
	tt := c.p.R.Types
	return c.typeHoldsUntil(t, func(u sem.TypeID) bool {
		return tt.Kind(u) == sem.KIface || tt.Kind(u) == sem.KNamed && tt.Node(u).Ent == tt.FutureEnt() || c.p.R.IsTaskType(u)
	}, func(u sem.TypeID) bool { return tt.Kind(u) == sem.KFn || tt.Kind(u) == sem.KClosure })
}

// heldAsync uses the argument type: a generic or interface parameter can hide the holder's mutable destinations.
func (c *borrowCheck) heldAsync(a LocalID) bool {
	return c.heldAsyncType(c.f.Locals[a].Type)
}

func (c *borrowCheck) heldAsyncType(t sem.TypeID) bool {
	return c.hidesLayout(t) || c.mutHolder(t) && c.typeHoldsUntil(t, func(u sem.TypeID) bool {
		elem, mut := c.p.R.MutRefElem(u)
		return mut && (c.asyncType(elem) || c.hidesLayout(elem))
	}, func(u sem.TypeID) bool {
		k := c.p.R.Types.Kind(u)
		return k == sem.KFn || k == sem.KClosure || k == sem.KIface
	})
}

func (c *borrowCheck) keptPlace(a LocalID) bool {
	if c.asyncPlace(a) {
		return true
	}
	t := c.placeOf(a)
	if t == 0 {
		t = c.f.Locals[a].Type
		if c.p.R.Types.Kind(t) == sem.KRef {
			t = c.p.R.Types.Node(t).Elem
		}
	}
	return c.heldAsyncType(t)
}

func (c *borrowCheck) keptTie(info *sem.FnInfo, q sem.EntityID) tieKind {
	t := c.p.R.Entity(q).Type
	switch {
	case q == info.SelfParam && info.Recv == sem.RecvMut:
		return tieRecvMut
	case q == info.SelfParam && info.Recv == sem.RecvSelf:
		return tieRecv
	case c.p.R.Types.Kind(t) == sem.KRef:
		return tieRef
	case q == info.SelfParam || c.holdsBorrow(t) || c.holdsDeferred(t) || c.namesTypeParam(t):
		return tieValue
	}
	return tieNone
}

func (c *borrowCheck) mutTarget(info *sem.FnInfo, q sem.EntityID) bool {
	if q == info.SelfParam {
		return info.Recv == sem.RecvMut
	}
	return c.p.R.Types.RefMut(c.p.R.Entity(q).Type)
}

// lentByCaller reports whether q, a parameter of the fn info describes, is lent by its caller: a reference or a `self` or
// `mut self` receiver.
func (c *borrowCheck) lentByCaller(info *sem.FnInfo, q sem.EntityID) bool {
	if q == info.SelfParam {
		return info.Recv == sem.RecvSelf || info.Recv == sem.RecvMut
	}
	return c.p.R.Types.Kind(c.p.R.Entity(q).Type) == sem.KRef
}
