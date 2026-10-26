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

// keptFlows ties into each `&mut` or `mut self` argument whose place can hold a Future or Task (asyncPlace) the other
// arguments the callee's keptSummary stores there, or the contents it moves there from a lent one.
func (c *borrowCheck) keptFlows(in Inst) []storeFlow {
	ents, args := []sem.EntityID{in.Ent}, in.Args
	if in.Op == OpCallValue {
		if in.Ent != 0 {
			return nil
		}
		var ok bool
		args = in.Args[1:]
		if ents, ok = c.fnItemOrigins(in.Args[0], 8); !ok || slices.ContainsFunc(ents, c.unsummarized) {
			return c.opaqueKeptFlows(args)
		}
	}
	var out []storeFlow
	for _, e := range ents {
		s := c.keptBound(e, c.siteBind(e, args))
		if s == nil || c.p.R.Entity(e).Kind != sem.EntFn || c.p.isAsyncBody(e) {
			continue
		}
		info, params := c.p.R.Fn(e), c.declaredParams(e)
		for t, ta := range args {
			if t >= len(params) || !c.mutTarget(info, params[t]) || !c.asyncPlace(c.viewSource(ta)) {
				continue
			}
			fl := storeFlow{target: c.viewSource(ta), ref: true, kept: true}
			for k, a := range args {
				tie := tieNone
				if k != t && k < len(params) && s.storesInto(t, k) {
					tie = c.keptTie(info, params[k])
				}
				switch {
				case tie != tieNone && (in.Op == OpCallValue || tie != tieValue):
					fl.values = append(fl.values, flowArg{arg: a, tie: tie})
				case k != t && k < len(params) && s.movesInto(t, k) && c.lentByCaller(info, params[k]):
					fl.values = append(fl.values, flowArg{arg: c.viewSource(a), contents: true})
				}
			}
			if fl.values != nil {
				out = append(out, fl)
			}
		}
	}
	return out
}

// opaqueKeptFlows is keptFlows for a value call of a callee with no keptSummary: every other argument may be stored.
func (c *borrowCheck) opaqueKeptFlows(args []LocalID) []storeFlow {
	var out []storeFlow
	for t, ta := range args {
		if _, mut := c.p.R.MutRefElem(c.f.Locals[ta].Type); !mut || !c.asyncPlace(c.viewSource(ta)) {
			continue
		}
		fl := storeFlow{target: c.viewSource(ta), ref: true, kept: true}
		for k, a := range args {
			switch {
			case k == t:
			case c.p.R.Types.Kind(c.f.Locals[a].Type) == sem.KRef:
				fl.values = append(fl.values, flowArg{arg: a, tie: tieRef})
			default:
				fl.values = append(fl.values, flowArg{arg: a, tie: tieValue})
			}
		}
		out = append(out, fl)
	}
	return out
}

func (c *borrowCheck) unsummarized(e sem.EntityID) bool {
	return c.p.R.Entity(e).Kind != sem.EntFn || !c.p.isAsyncBody(e) && c.keptOf(e) == nil
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
	tt := c.p.R.Types
	return c.typeHoldsUntil(t, func(u sem.TypeID) bool {
		return tt.Kind(u) == sem.KIface || tt.Kind(u) == sem.KNamed && tt.Node(u).Ent == tt.FutureEnt() || c.p.R.IsTaskType(u)
	}, func(u sem.TypeID) bool { return tt.Kind(u) == sem.KFn || tt.Kind(u) == sem.KClosure })
}

// viewSource is what an existential view a lends boxes, whose place a store through the view reaches; else a itself.
func (c *borrowCheck) viewSource(a LocalID) LocalID {
	if d := c.definition(a); d != nil && d.Op == OpLendBox && len(d.Args) > 0 {
		return d.Args[0]
	}
	return a
}

// withKeptFlows adds keptFlows to fls, each keeping only the values no flow into the same target already names, as a
// flow of its own so it stays kept.
func (c *borrowCheck) withKeptFlows(in Inst, fls []storeFlow) []storeFlow {
	n := len(fls)
	for _, k := range c.keptFlows(in) {
		k.values = slices.DeleteFunc(k.values, func(v flowArg) bool {
			return slices.ContainsFunc(fls[:n], func(f storeFlow) bool {
				return f.target == k.target && slices.ContainsFunc(f.values, func(h flowArg) bool { return h.arg == v.arg })
			})
		})
		if len(k.values) > 0 {
			fls = append(fls, k)
		}
	}
	return fls
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
