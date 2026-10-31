package borrow

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// A callee without a body may store only when the call hands it a value that carries a borrow.
func (c *borrowCheck) calleeStoreSites(fl storeFlow) []paramStore {
	if fl.callee == 0 && !fl.opaque {
		return nil
	}
	if g := c.p.ByEnt[fl.callee]; g != nil && !fl.opaque {
		sites := c.paramStoreSites(g, fl.param)
		if !slices.ContainsFunc(sites, func(s paramStore) bool { return c.quietCallee(fl, s) }) {
			return sites
		}
		return slices.DeleteFunc(slices.Clone(sites), func(s paramStore) bool { return c.quietCallee(fl, s) })
	}
	if slices.ContainsFunc(fl.values, func(v flowArg) bool { return c.carries(c.f.Locals[v.arg].Type) }) {
		callback := slices.ContainsFunc(fl.values, func(v flowArg) bool { return c.callableArg(v.arg) && c.callableMayStore(v.arg) })
		return []paramStore{{via: fl.opaque || callback || c.seesThrough(fl.callee, fl.param)}}
	}
	return nil
}

func (c *borrowCheck) seesThrough(callee sem.EntityID, slot int) bool {
	if callee == 0 || c.p.R.Entity(callee).Kind != sem.EntFn {
		return true
	}
	info := c.p.R.Fn(callee)
	var decl sem.TypeID
	switch {
	case info.Recv != sem.RecvNone && slot == 0:
		decl = c.p.R.Entity(info.SelfParam).Type
	case info.Recv != sem.RecvNone:
		slot--
		fallthrough
	default:
		if slot >= len(info.Params) {
			return true
		}
		decl = c.p.R.Entity(info.Params[slot]).Type
	}
	if c.p.R.Types.Kind(decl) == sem.KRef {
		decl = c.p.R.Types.Node(decl).Elem
	}
	return decl == 0 || c.storesMutRef(decl)
}

// lentContents drops the exclusive borrows from another `&mut` argument's contents when the callee cannot take
// anything out of that parameter: such a borrow cannot be copied, so only a move can bring it to the target.
func (c *borrowCheck) lentContents(fl storeFlow, v flowArg, b *borrowInfo) *borrowInfo {
	if g := c.p.ByEnt[fl.callee]; !v.contents || v.at != nil || v.param < 0 || g == nil || fl.opaque || !b.hasOwner || c.paramMoves(g, v.param) {
		return b
	}
	out := *b
	out.owners = slices.DeleteFunc(slices.Clone(b.owners), func(o ownerRef) bool { return o.mut })
	return &out
}

// quietCallee reports a site that hands the value to code the call passes, when that code stores through nothing.
func (c *borrowCheck) quietCallee(fl storeFlow, s paramStore) bool {
	return s.callee.kind == relayParam && s.callee.param < len(fl.args) && c.callableArg(fl.args[s.callee.param]) && !c.callableMayStore(fl.args[s.callee.param])
}
