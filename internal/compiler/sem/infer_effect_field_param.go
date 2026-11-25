package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// FieldParamKey names a `pure?` field read through one of a function's own
// parameters for field-argument propagation.
type FieldParamKey struct {
	Param EntityID
	Field EntityID
}

// identParam excludes a closure body, whose summary is trusted once with
// no per-call re-instantiation to defer to, and a `mut` parameter,
// which has no entry-point fact a reassignment could invalidate.
func (c *checker) identParam(n syntax.NodeID) EntityID {
	if c.t.Kind(n) != syntax.Ident || len(c.lambdas) > 0 {
		return 0
	}
	p := c.info.Uses[n]
	if p == 0 || c.r.Entities[p].Kind != EntParam || c.r.Entities[p].Parent != c.fn {
		return 0
	}
	if c.r.Entities[p].Flags&EfMut != 0 {
		return 0
	}
	return p
}

// deferToFieldParam implements field-argument propagation.
func (c *checker) deferToFieldParam(p, field EntityID) {
	info := c.r.Fn(c.fn)
	key := FieldParamKey{Param: p, Field: field}
	c.r.markDeferred(c.r.fieldParamEnt(fieldFnKey{Ent: p, Field: field}), c.viaCopy)
	if !slices.Contains(info.CalledFieldParams, key) {
		info.CalledFieldParams = append(info.CalledFieldParams, key)
	}
}

// fieldCallbackEdges implements field-argument propagation.
func (c *checker) fieldCallbackEdges(fn EntityID, args []syntax.NodeID) {
	info := c.r.Fn(fn)
	need := info.Declared & (EffPure | EffNoalloc)
	for _, key := range info.CalledFieldParams {
		i := slices.Index(info.Params, key.Param)
		if i < 0 || i >= len(args) || c.t.Kind(args[i]) == syntax.Spread {
			continue
		}
		c.fieldCallbackEdge(args[i], key.Field, need)
	}
}

// fieldCallbackEdge is effectFact's call-site counterpart: it resolves an
// actual argument expression instead of a record literal's field initializer.
// need is the callee's own declared contract: a field it invokes must prove it
// here, since no later caller can.
func (c *checker) fieldCallbackEdge(expr syntax.NodeID, field EntityID, need Effects) {
	arg := c.fieldArgOperand(expr)
	if c.t.Kind(arg) == syntax.RecordLit {
		if fact, ok := c.recordLitFieldFact(arg, field); ok {
			c.applyFieldFact(expr, fact, need)
			return
		}
	} else if place := c.placeOf(arg); place != 0 {
		fp := c.r.internPlace(c.r.fieldPlace(c.r.Places[place], field))
		for i := len(c.facts) - 1; i >= 0; i-- {
			if f := c.facts[i]; f.Kind == FactPureField && f.Place == fp {
				c.applyFieldFact(expr, f, need)
				return
			}
		}
		if p := c.identParam(arg); p != 0 {
			c.deferToFieldParam(p, field)
			c.edge(EffectEdge{Kind: EdgeParamCall, Target: p, Node: expr})
			return
		}
	}
	c.edge(EffectEdge{Kind: EdgeCallback, Node: expr})
	c.deferredValueContract(expr, 0, need)
}

// fieldArgOperand strips the borrow and parentheses an argument adds, so
// `call(&r)` and `call(r)` resolve r's field the same way.
func (c *checker) fieldArgOperand(n syntax.NodeID) syntax.NodeID {
	for {
		switch c.t.Kind(n) {
		case syntax.Paren:
			n = c.t.View(n).AsParen().Inner().ID()
		case syntax.BorrowExpr:
			n = c.t.View(n).AsBorrow().Operand().ID()
		default:
			return n
		}
	}
}

// deferredValueContract reports a fn value whose effects are known to fall
// short of a `pure`/`noalloc` callee's own declaration. A proven fn item or
// closure goes through EdgeContract instead (see applyFieldFact).
func (c *checker) deferredValueContract(n syntax.NodeID, have, need Effects) {
	for _, mod := range []Effects{EffPure, EffNoalloc} {
		if need&mod == 0 || have&mod != 0 {
			continue
		}
		if mod == EffPure {
			c.errAt(n, cImpureCallback, c.r.exprName(c.f, n))
		} else {
			c.errAt(n, cAllocCallback, c.r.exprName(c.f, n))
		}
	}
}

// recordLitFieldFact resolves a record literal directly, without a bound
// place (e.g. `useApp(App{view: f})`).
func (c *checker) recordLitFieldFact(lit syntax.NodeID, field EntityID) (NarrowFact, bool) {
	call, ok := c.info.Calls[lit]
	if !ok || call.Kind != CallRecord {
		return NarrowFact{}, false
	}
	for entry := range c.t.View(lit).AsRecordLit().Fields().IDs() {
		if c.t.Kind(entry) == syntax.Spread || c.info.Uses[entry] != field {
			continue
		}
		return c.effectFact(c.t.View(entry).AsFieldInit().Value().ID())
	}
	return NarrowFact{}, false
}
