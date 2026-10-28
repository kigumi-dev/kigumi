package mir

import (
	"slices"

	"kigumi/internal/sem"
)

func (c *borrowCheck) keptStoreFlows(e sem.EntityID, args []LocalID, s *keptSummary, t int) []storeFlow {
	info, params := c.p.R.Fn(e), c.declaredParams(e)
	if t >= len(params) {
		return nil
	}
	targets := c.keptTargets(info, params[t], args[t])
	var values []flowArg
	for k, a := range args {
		if k == t || k >= len(params) {
			continue
		}
		tie := tieNone
		if s.storesInto(t, k) {
			tie = c.keptTie(info, params[k])
		}
		switch {
		case tie != tieNone:
			values = append(values, flowArg{arg: a, tie: tie, param: k})
		case s.movesInto(t, k) && c.lentByCaller(info, params[k]):
			for _, source := range c.viewSources(a).locals {
				values = append(values, flowArg{arg: source, contents: true, param: k})
			}
		}
	}
	for i := range targets {
		targets[i].callee, targets[i].param, targets[i].args = e, t, args
		targets[i].values = slices.Clone(values)
	}
	return targets
}

func (c *borrowCheck) unknownKeptValues(e sem.EntityID, args []LocalID, values []flowArg) []flowArg {
	callback := slices.ContainsFunc(args, func(a LocalID) bool {
		return c.carriesMaker(c.f.Locals[a].Type, map[sem.TypeID]bool{}, c.holdsAsync)
	})
	if callback {
		return values
	}
	slots := c.callSlots(c.p.R.Fn(e), args, 0)
	for i := range values {
		v := &values[i]
		if v.param < len(slots) && slots[v.param].contents {
			v.tie, v.contents = tieNone, true
		}
	}
	return values
}
