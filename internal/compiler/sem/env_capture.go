package sem

import (
	"maps"
	"slices"
)

// whyEnvErase marks the box erasing a lambda into an fn value takes for the runtime types it captures; it stays
// only if the lambda needs one at runtime.
const whyEnvErase = 26

// envUses are, per body, the runtime type operations it performs on types it names and the instantiations of the
// bodies it calls or makes values of; and, per lambda, the parameters it can only know at runtime.
type envUses struct {
	ops   map[EntityID][]TypeID
	calls map[EntityID][]rtCall
	outer map[EntityID]map[EntityID]bool
	// needs are the bodies settleEnvCaptures found needing a runtime type.
	needs map[EntityID]bool
}

type rtCall struct {
	fn   EntityID
	args []TypeID
}

func (c *checker) body() EntityID {
	if len(c.lambdas) > 0 {
		return c.lambdas[len(c.lambdas)-1].ent
	}
	return c.fn
}

// rtRef is a use recorded in the current full expression, whose types settle when it ends.
type rtRef struct {
	body EntityID
	call bool
	i    int
}

// rtUse records that the body being checked composes runtime type t: a type test or a lent box of it.
func (c *checker) rtUse(t TypeID) {
	u := &c.r.envUse
	if u.ops == nil {
		u.ops = map[EntityID][]TypeID{}
	}
	b := c.body()
	u.ops[b] = append(u.ops[b], t)
	c.rtPending = append(c.rtPending, rtRef{b, false, len(u.ops[b]) - 1})
}

// calleeUse is the substitution a use of fn instantiates it with, its receiver's arguments included: a call, a fn
// or method value, a witness bound into one. Every such use goes through here, so the capture fixpoint sees each.
func (c *checker) calleeUse(fn EntityID, inst []TypeID, recv TypeID) map[EntityID]TypeID {
	subst := c.callSubst(fn, inst, recv)
	c.rtCall(c.body(), fn, slices.Collect(maps.Values(subst)))
	return subst
}

// rtCall records that body calls or makes a value of fn at the instantiation args.
func (c *checker) rtCall(body, fn EntityID, args []TypeID) {
	if len(args) == 0 {
		return
	}
	u := &c.r.envUse
	if u.calls == nil {
		u.calls = map[EntityID][]rtCall{}
	}
	u.calls[body] = append(u.calls[body], rtCall{fn, slices.Clone(args)})
	c.rtPending = append(c.rtPending, rtRef{body, true, len(u.calls[body]) - 1})
}

func (c *checker) settleRtUses() {
	u := &c.r.envUse
	for _, ref := range c.rtPending {
		if !ref.call {
			u.ops[ref.body][ref.i] = c.vars.resolve(u.ops[ref.body][ref.i])
			continue
		}
		args := u.calls[ref.body][ref.i].args
		for k := range args {
			args[k] = c.vars.resolve(args[k])
		}
	}
	c.rtPending = c.rtPending[:0]
}

// noteLambdaOuter keeps the parameters lambda id can only know at runtime: those of the generic it is written in and
// those tests before it opened.
func (c *checker) noteLambdaOuter(id EntityID, mark int) {
	outer := c.outerParams(mark)
	if len(outer) == 0 {
		return
	}
	u := &c.r.envUse
	if u.outer == nil {
		u.outer = map[EntityID]map[EntityID]bool{}
	}
	u.outer[id] = outer
}

func (c *checker) outerParams(mark int) map[EntityID]bool {
	outer := map[EntityID]bool{}
	for _, p := range c.opens[:mark] {
		outer[p] = true
	}
	if fn := c.enclosingFn(); fn != 0 {
		info := c.r.Fn(fn)
		for _, p := range append(append([]EntityID(nil), info.TypeParams...), info.RecvParams...) {
			outer[p] = !c.r.typeParam(p).IsLifetime
		}
	}
	return outer
}

// noteMethodClosure records closure id, a method value of m taking its receiver of type recv, as a lambda calling m.
func (c *checker) noteMethodClosure(id, m EntityID, recv TypeID) {
	c.noteLambdaOuter(id, len(c.opens))
	c.rtCall(id, m, []TypeID{recv})
}

func (c *checker) enclosingFn() EntityID {
	for ent := c.fn; ent != 0 && c.r.Entities[ent].Kind != EntPackage; ent = c.r.Entities[ent].Parent {
		if c.r.Entities[ent].Kind == EntFn {
			return ent
		}
	}
	return 0
}

// settleEnvCaptures drops the erasure boxes of lambdas needing no runtime type: a body needs one when it composes a
// type naming its parameters, calls such a body at an instantiation naming them, or builds a lambda that does.
func (r *Result) settleEnvCaptures(nodes []EntityID) {
	needs := map[EntityID]bool{}
	kids := map[EntityID][]EntityID{}
	for _, id := range nodes {
		if r.Entities[id].Kind == EntClosure {
			kids[r.closure(id).Creator] = append(kids[r.closure(id).Creator], id)
		}
	}
	for changed := true; changed; {
		changed = false
		for _, id := range nodes {
			if !needs[id] && r.needsRuntimeTypes(id, needs, kids[id]) {
				needs[id], changed = true, true
			}
		}
	}
	for _, id := range nodes {
		edges := r.edgesPtr(id)
		*edges = slices.DeleteFunc(*edges, func(e EffectEdge) bool {
			return e.Why == whyEnvErase && !needs[r.Types.Node(e.From).Ent]
		})
	}
	r.envUse.needs = needs
}

// CapturesRuntimeTypes reports whether lambda cl, or the lambda it was copied from, reads a runtime type it must
// capture: what the backends' environment pass may add to it, and sem charged for.
func (r *Result) CapturesRuntimeTypes(cl EntityID) bool { return r.envUse.needs[r.ClosureOrigin(cl)] }

func (r *Result) needsRuntimeTypes(id EntityID, needs map[EntityID]bool, kids []EntityID) bool {
	params := r.runtimeParams(id)
	if len(params) == 0 {
		return false
	}
	seen := map[EntityID]bool{}
	names := func(t TypeID) bool { return r.namesAny(t, params, seen) }
	if slices.ContainsFunc(r.envUse.ops[id], names) || slices.ContainsFunc(kids, func(k EntityID) bool { return needs[k] }) {
		return true
	}
	for _, call := range r.envUse.calls[id] {
		if (needs[call.fn] || r.IsErrorAs(call.fn)) && slices.ContainsFunc(call.args, names) {
			return true
		}
	}
	for _, e := range r.edgesOf(id) {
		switch {
		case e.Kind == EdgeAlloc && e.From != 0 && e.Why != whyEnvErase && names(e.From):
			return true
		case e.Kind == EdgeDrop && names(e.Type) && needs[r.dropOf(e.Type)]:
			return true
		case e.Kind == EdgeWitness && needs[e.Target]:
			return true
		}
	}
	return false
}

// runtimeParams are the parameters body id knows only at runtime: a generic fn's own, or what a lambda named
// from outside it.
func (r *Result) runtimeParams(id EntityID) map[EntityID]bool {
	if r.Entities[id].Kind == EntClosure {
		return r.envUse.outer[id]
	}
	if r.Entities[id].Kind != EntFn {
		return nil
	}
	info := r.Fn(id)
	out := map[EntityID]bool{}
	for _, p := range append(append([]EntityID(nil), info.TypeParams...), info.RecvParams...) {
		if !r.typeParam(p).IsLifetime {
			out[p] = true
		}
	}
	return out
}

// IsErrorAs reports whether fn is std/error's `as`, whose instantiation the runtime reads back as a type.
func (r *Result) IsErrorAs(fn EntityID) bool { return fn != 0 && fn == r.langItem(0, 0, "as") }

// dropOf is the drop fn of t's declaration, 0 for none.
func (r *Result) dropOf(t TypeID) EntityID {
	if n := r.Types.Node(t); n.Kind == KNamed && r.Entities[n.Ent].Kind == EntType {
		return r.typeDecl(n.Ent).Drop
	}
	return 0
}
