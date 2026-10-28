package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// keptFlows ties into each keptTarget of a call the other arguments the callee's keptSummary stores there, or the
// contents it moves there from a lent one.
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
			if t >= len(params) {
				continue
			}
			fl, ok := c.keptTarget(info, params[t], ta)
			if !ok {
				continue
			}
			if fl.byValue {
				fl.callee, fl.param, fl.args = e, t, args
			}
			for k, a := range args {
				tie := tieNone
				if k != t && k < len(params) && s.storesInto(t, k) {
					tie = c.keptTie(info, params[k])
				}
				switch {
				case tie != tieNone && (in.Op == OpCallValue || tie != tieValue):
					fl.values = append(fl.values, flowArg{arg: a, tie: tie, param: k})
				case k != t && k < len(params) && s.movesInto(t, k) && c.lentByCaller(info, params[k]):
					fl.values = append(fl.values, flowArg{arg: c.viewSource(a), contents: true, param: k})
				}
			}
			if fl.values != nil {
				out = append(out, fl)
			}
		}
	}
	return out
}

// keptTarget is the kept flow into a, the argument for q: a `&mut` or `mut self` whose place is an asyncPlace, or a
// holder handed over by value whose `&mut`s reach one (heldAsync), which lands where the callee stores through it.
func (c *borrowCheck) keptTarget(info *sem.FnInfo, q sem.EntityID, a LocalID) (storeFlow, bool) {
	switch {
	case c.mutTarget(info, q):
		return storeFlow{target: c.viewSource(a), ref: true, kept: true}, c.asyncPlace(c.viewSource(a))
	case c.byValueParam(info, q) && c.heldAsync(a):
		return storeFlow{target: a, byValue: true, kept: true}, true
	}
	return storeFlow{}, false
}

func (c *borrowCheck) byValueParam(info *sem.FnInfo, q sem.EntityID) bool {
	if q == info.SelfParam {
		return info.Recv == sem.RecvMove
	}
	return c.p.R.Types.Kind(c.p.R.Entity(q).Type) != sem.KRef
}

// opaqueKeptFlows is keptFlows for a value call of a callee with no keptSummary: every other argument may be stored.
func (c *borrowCheck) opaqueKeptFlows(args []LocalID) []storeFlow {
	var out []storeFlow
	for t, ta := range args {
		var fl storeFlow
		switch _, mut := c.p.R.MutRefElem(c.f.Locals[ta].Type); {
		case mut && c.asyncPlace(c.viewSource(ta)):
			fl = storeFlow{target: c.viewSource(ta), ref: true, kept: true}
		case !mut && c.heldAsync(ta):
			fl = storeFlow{target: ta, byValue: true, kept: true, opaque: true}
		default:
			continue
		}
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

// withKeptFlows adds keptFlows to fls, each keeping only the values no flow into the same target already stores the
// same way (the argument itself, or what it lends), as a flow of its own so it stays kept.
func (c *borrowCheck) withKeptFlows(in Inst, fls []storeFlow) []storeFlow {
	n := len(fls)
	for _, k := range c.keptFlows(in) {
		k.values = slices.DeleteFunc(k.values, func(v flowArg) bool {
			return slices.ContainsFunc(fls[:n], func(f storeFlow) bool {
				return f.target == k.target && slices.ContainsFunc(f.values, func(h flowArg) bool { return h.arg == v.arg && h.contents == v.contents })
			})
		})
		if len(k.values) > 0 {
			fls = append(fls, k)
		}
	}
	return fls
}
