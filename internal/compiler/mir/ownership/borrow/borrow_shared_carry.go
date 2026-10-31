package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// carryMode is what a call's result may take from one operand: the operand's own value, what its referent holds, or a
// borrow of the place it names.
type carryMode uint8

const (
	carryValue carryMode = 1 << iota
	carryContents
	carryPlace
)

// resultCarries says per operand what call in's result may take, by the callee's keptSummary or, for a bodiless fn, by
// its declared shape: an owned result naming no lifetime and holding nothing deferred takes nothing; false when unknown.
func (c *borrowCheck) resultCarries(in midir.Inst) (func(int) carryMode, bool) {
	e := in.Ent
	if in.Op == midir.OpCallValue {
		e = c.calleeClosure(c.f, in)
	}
	fn := e
	if m := c.methodOf(e); m != 0 {
		fn = m
	}
	if fn == 0 || c.p.R.Entity(fn).Kind != sem.EntFn || ownquery.IsAsyncBody(c.p.R, fn) || c.ifaceRequirement(fn) {
		return nil, false
	}
	param := func(k int) int {
		if q := c.boundParams([]sem.EntityID{e}, in, k); len(q) > 0 {
			return slices.Index(c.declaredParams(fn), q[0])
		}
		return -1
	}
	if c.p.R.Fn(fn).Body != 0 {
		s := c.keptBound(fn, c.siteBind(fn, in.Args))
		if s == nil {
			return nil, false
		}
		return func(k int) carryMode {
			switch j := param(k); {
			case j < 0:
				return 0
			case s.keepsResult(j):
				return carryValue | carryPlace
			case s.movesOut(j):
				return carryContents
			}
			return 0
		}, true
	}
	return c.primitiveCarries(in, fn, param), true
}

// primitiveCarries is resultCarries for bodiless fn: a reference result may borrow a lent operand's place, a result that
// can hold a deferred value takes by-value operands and lent contents, and a callback keeping its argument borrows the rest.
func (c *borrowCheck) primitiveCarries(in midir.Inst, fn sem.EntityID, param func(int) int) func(int) carryMode {
	t := c.f.Locals[in.Dst].Type
	refs, holds := c.facts().HoldsBorrow(t), c.facts().HoldsBorrow(t) || c.holdsDeferred(t) || c.facts().NamesTypeParam(t)
	params := c.declaredParams(fn)
	lent := func(k int) bool {
		j := param(k)
		return c.p.R.Types.Kind(c.f.Locals[in.Args[k]].Type) == sem.KRef || c.f.Locals[in.Args[k]].Cell ||
			j == 0 && c.p.R.Fn(fn).Recv != sem.RecvNone && c.p.R.Fn(fn).Recv != sem.RecvMove
	}
	kept := slices.ContainsFunc(in.Args, func(a midir.LocalID) bool { return c.callableKeeps(a) })
	return func(k int) carryMode {
		if !holds || param(k) < 0 || param(k) >= len(params) {
			return 0
		}
		m := carryValue
		if lent(k) {
			m = carryContents
			if refs || kept {
				m |= carryPlace
			}
		}
		return m
	}
}

// callableKeeps reports a callable operand whose result may keep an argument it is given: an async fn, or a fn or closure
// whose keptSummary says so; an unknown one is taken to.
func (c *borrowCheck) callableKeeps(a midir.LocalID) bool {
	t := c.f.Locals[a].Type
	if !c.callableNode(t) {
		return false
	}
	ents, ok := c.fnItemOrigins(a, 8)
	if n := c.p.R.Types.Node(t); n.Kind == sem.KClosure && n.Ent != 0 && c.p.R.Entity(n.Ent).Kind == sem.EntClosure {
		ents, ok = []sem.EntityID{n.Ent}, true
	}
	return !ok || slices.ContainsFunc(ents, func(e sem.EntityID) bool {
		if c.p.R.Entity(e).Kind == sem.EntFn && (ownquery.IsAsyncBody(c.p.R, e) || c.p.R.Fn(e).Body == 0) {
			return true
		}
		s := c.keptOf(e)
		return s == nil || slices.ContainsFunc(c.declaredParams(e), func(q sem.EntityID) bool {
			return s.keepsResult(slices.Index(c.declaredParams(e), q))
		})
	})
}
