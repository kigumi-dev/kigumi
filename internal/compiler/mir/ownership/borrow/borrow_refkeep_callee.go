package borrow

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// calleeRefKeeps is refKeeps, or for a callee without a body bodylessRefKeeps; never nil.
func (c *borrowCheck) calleeRefKeeps(e sem.EntityID, bind fnBind) *refSummary {
	if s := c.refKeeps(e, bind); s != nil {
		return s
	}
	return c.bodylessRefKeeps(e, bind)
}

// bodylessRefKeeps keeps everything when some callable e is passed, declared or a witness, may keep anything: by its
// summaries when bound, by valueKeeps on the parameter's type when not; else nothing.
func (c *borrowCheck) bodylessRefKeeps(e sem.EntityID, bind fnBind) *refSummary {
	if c.p.R.Entity(e).Kind != sem.EntFn {
		return &refSummary{all: true}
	}
	var slots []sem.TypeID
	for _, q := range c.declaredParams(e) {
		slots = append(slots, c.p.R.Entity(q).Type)
	}
	params := len(slots)
	for _, w := range c.hiddenLocals(e) {
		slots = append(slots, c.p.R.Entity(w).Type)
	}
	for j, t := range slots {
		if j < params && c.slotFnType(t) == 0 {
			continue
		}
		fns, bound := bind[j]
		if !bound && c.valueKeeps(t).keepsAny() {
			return &refSummary{all: true}
		}
		for _, f := range fns {
			s := c.calleeRefKeeps(f.ent, f.sub)
			if c.p.R.Entity(f.ent).Kind == sem.EntFn {
				s = s.join(c.declaredRefKeeps(f.ent))
			}
			if s.keepsAny() {
				return &refSummary{all: true}
			}
		}
	}
	return &refSummary{}
}

// valueKeeps is what a call of an unknown value of type t keeps: every argument's borrows, in a result and in each
// `&mut` or holder argument that can hold one; a Future or Task result keeps them through the kept flows instead.
func (c *borrowCheck) valueKeeps(t sem.TypeID) *refSummary {
	ft := c.slotFnType(t)
	if ft == 0 {
		return &refSummary{all: true}
	}
	n := c.p.R.Types.Node(ft)
	var every refBits
	for j := range n.Args {
		every = every.with(refBit(j, refOwn)).with(refBit(j, refHeld))
	}
	s := &refSummary{stores: map[int]refBits{}, excl: every}
	if c.mayHold(n.Elem) {
		s.result = every
	}
	for j, a := range n.Args {
		elem, mut := c.p.R.MutRefElem(a)
		if mut && c.mayHold(elem) || c.mutHolder(a) {
			s.stores[j] = every
		}
	}
	return s
}

func (c *borrowCheck) mayHold(t sem.TypeID) bool {
	return c.facts().HoldsBorrow(t) || c.facts().NamesTypeParam(t) || c.hidesLayout(t)
}

// declaredRefKeeps is what e's signature lets it keep of a reference parameter's own borrow: one whose own named
// lifetime its result or the type of a `&mut` / `mut self` parameter names again.
func (c *borrowCheck) declaredRefKeeps(e sem.EntityID) *refSummary {
	info, params := c.p.R.Fn(e), c.declaredParams(e)
	ret := c.p.R.Types.Node(info.Sig).Elem
	s := &refSummary{stores: map[int]refBits{}}
	for j, q := range params {
		if q == info.SelfParam || c.p.R.Local(q).Lifetime == 0 {
			continue
		}
		lt, bit := c.p.R.Local(q).Lifetime, refBit(j, refOwn)
		if c.p.R.Types.RefMut(c.p.R.Entity(q).Type) {
			s.excl = s.excl.with(bit)
		}
		if info.RetLifetime == lt || slices.Contains(c.p.R.Types.Params(ret), lt) {
			s.result = s.result.with(bit)
		}
		for t, p := range params {
			if c.mutTarget(info, p) && slices.Contains(c.p.R.Types.Params(c.p.R.Entity(p).Type), lt) {
				s.stores[t] = s.stores[t].with(bit)
			}
		}
	}
	return s
}
