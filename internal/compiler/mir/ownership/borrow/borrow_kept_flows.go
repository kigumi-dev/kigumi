package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) keptFlows(in midir.Inst) []storeFlow {
	ents, args := c.keptFlowCallees(in)
	witness := c.ifaceRequirement(in.Ent)
	unknown := len(ents) == 0 || !witness && (in.Op == midir.OpCallValue || in.Op == midir.OpFutureNew) && slices.ContainsFunc(ents, c.unsummarized)
	unknown = unknown || witness && len(c.implementers(in.Ent)) == 0
	if unknown {
		if witness {
			var out []storeFlow
			for t := range args {
				for _, fl := range c.keptStoreFlows(in.Ent, args, &keptSummary{all: true}, t) {
					fl.values = c.unknownKeptValues(in.Ent, args, fl.values)
					fl.opaque = true
					out = append(out, fl)
				}
			}
			return out
		}
		return c.opaqueKeptFlows(args)
	}
	var out []storeFlow
	for _, e := range ents {
		if c.p.R.Entity(e).Kind != sem.EntFn {
			continue
		}
		s := c.keptStoreSummary(e, c.siteBind(e, args))
		opaque := s == nil
		if opaque {
			if !witness {
				continue
			}
			s = &keptSummary{all: true}
		}
		for t := range args {
			for _, fl := range c.keptStoreFlows(e, args, s, t) {
				fl.opaque = opaque
				if in.Op == midir.OpCall && fl.byValue {
					fl.values = slices.DeleteFunc(fl.values, func(v flowArg) bool { return v.tie == tieValue })
				}
				if len(fl.values) > 0 {
					out = append(out, fl)
				}
			}
		}
	}
	return out
}

func (c *borrowCheck) keptTargets(info *sem.FnInfo, q sem.EntityID, a midir.LocalID) []storeFlow {
	decl := c.p.R.Entity(q).Type
	if c.p.R.Types.Kind(decl) == sem.KRef {
		decl = c.p.R.Types.Node(decl).Elem
	}
	switch {
	case c.mutTarget(info, q) && (c.asyncType(decl) || c.heldAsyncType(decl)):
		return c.keptMutTargets(a)
	case c.byValueParam(info, q) && c.heldAsync(a):
		return []storeFlow{{target: a, byValue: true, kept: true}}
	}
	return nil
}

func (c *borrowCheck) byValueParam(info *sem.FnInfo, q sem.EntityID) bool {
	if q == info.SelfParam {
		return info.Recv == sem.RecvMove
	}
	return c.p.R.Types.Kind(c.p.R.Entity(q).Type) != sem.KRef
}

func (c *borrowCheck) opaqueKeptFlows(args []midir.LocalID) []storeFlow {
	var out []storeFlow
	for t, ta := range args {
		var targets []storeFlow
		switch _, mut := c.p.R.MutRefElem(c.f.Locals[ta].Type); {
		case mut:
			targets = c.keptMutTargets(ta)
		case c.heldAsync(ta):
			targets = []storeFlow{{target: ta, byValue: true, kept: true}}
		}
		for _, fl := range targets {
			fl.opaque = true
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
	}
	return out
}

// withKeptFlows adds keptFlows to fls, each keeping only the values no flow into the same target already stores the
// same way (the argument itself, or what it lends), as a flow of its own so it stays kept.
func (c *borrowCheck) withKeptFlows(in midir.Inst, fls []storeFlow) []storeFlow {
	carried, _ := c.carryFlows(in)
	known := slices.Concat(fls, carried)
	for _, k := range c.keptFlows(in) {
		k.values = slices.DeleteFunc(k.values, func(v flowArg) bool {
			return !k.unresolved && slices.ContainsFunc(known, func(f storeFlow) bool {
				return f.target == k.target && slices.ContainsFunc(f.values, func(h flowArg) bool { return h.arg == v.arg && h.contents == v.contents })
			})
		})
		if len(k.values) > 0 {
			fls = append(fls, k)
		}
	}
	return fls
}
