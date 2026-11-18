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
	// builds are, per body, the types of the records it builds.
	builds map[EntityID][]TypeID
	outer  map[EntityID]map[EntityID]bool
	// needs are the bodies settleEnvCaptures found needing a runtime type, lanes those lowering a SIMD
	// operation whose lanes their parameters fix.
	needs, lanes map[EntityID]bool
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
	kind rtKind
	i    int
}

type rtKind uint8

const (
	rtOp rtKind = iota
	rtCallArgs
	rtBuild
)

// rtUse records that the body being checked composes runtime type t: a type test or a lent box of it.
func (c *checker) rtUse(t TypeID) { c.rtType(&c.r.envUse.ops, rtOp, t) }

// rtBuilt records that the body being checked builds a record of type t, whose drop may read t.
func (c *checker) rtBuilt(t TypeID) { c.rtType(&c.r.envUse.builds, rtBuild, t) }

func (c *checker) rtType(m *map[EntityID][]TypeID, kind rtKind, t TypeID) {
	if *m == nil {
		*m = map[EntityID][]TypeID{}
	}
	b := c.body()
	(*m)[b] = append((*m)[b], t)
	c.rtPending = append(c.rtPending, rtRef{b, kind, len((*m)[b]) - 1})
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
	c.rtPending = append(c.rtPending, rtRef{body, rtCallArgs, len(u.calls[body]) - 1})
}

func (c *checker) settleRtUses() {
	u := &c.r.envUse
	for _, ref := range c.rtPending {
		switch ref.kind {
		case rtOp:
			u.ops[ref.body][ref.i] = c.vars.resolve(u.ops[ref.body][ref.i])
		case rtBuild:
			u.builds[ref.body][ref.i] = c.vars.resolve(u.builds[ref.body][ref.i])
		case rtCallArgs:
			args := u.calls[ref.body][ref.i].args
			for k := range args {
				args[k] = c.vars.resolve(args[k])
			}
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
