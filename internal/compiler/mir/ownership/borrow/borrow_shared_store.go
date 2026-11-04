package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// sharedStore is one operand of a call whose value may land in shared storage the place checker cannot follow; param is
// the callee parameter it binds, 0 when the callee is unknown.
type sharedStore struct {
	arg   int
	param sem.EntityID
}

// sinkCtx is one body read for its stores into shared storage: its value flow, and the callables a call site binds its
// callable parameters to.
type sinkCtx struct {
	g    *midir.Func
	fs   flowSets
	bind map[int][]sem.EntityID
	defs map[midir.LocalID]*midir.Inst
}

// interiorParam reports a parameter of bodiless e that e may write into without exclusive access: shared storage by its
// declared layout, taken by `self`, by value or by `&`; a `mut self` or `&mut` write is one the place checker follows.
func (c *borrowCheck) interiorParam(e, q sem.EntityID) bool {
	info := c.p.R.Fn(e)
	if q == info.SelfParam && info.Recv != sem.RecvSelf {
		return false
	}
	if _, mut := c.p.R.MutRefElem(c.p.R.Entity(q).Type); mut {
		return false
	}
	return c.sharedStorage(c.p.R.Entity(q).Type)
}

// sharedStoreArgs are the operands call in may store into shared storage: by the callee's keptParams, or unknownStores
// for a callee x cannot name.
func (c *borrowCheck) sharedStoreArgs(x *sinkCtx, in midir.Inst) []sharedStore {
	if in.Op != midir.OpCall && in.Op != midir.OpCallValue && in.Op != midir.OpFutureNew {
		return nil
	}
	ts, known := c.sinkCallees(x, in)
	if !known {
		return c.unknownStores(x, in)
	}
	var out []sharedStore
	for _, t := range ts {
		kept := c.keptParams(x, in, t)
		for k := range in.Args {
			for _, q := range t.bind(k) {
				if slices.Contains(kept, q) && !slices.ContainsFunc(out, func(s sharedStore) bool { return s.arg == k }) {
					out = append(out, sharedStore{arg: k, param: q})
				}
			}
		}
	}
	return out
}

// keptParams are the parameters of e that call in may store into shared storage: a body's sharedSinks, or for a bodiless
// fn every keepable parameter beside an interiorParam that call in binds to shared storage.
func (c *borrowCheck) keptParams(x *sinkCtx, in midir.Inst, t sinkTarget) []sem.EntityID {
	e, fn := t.ent, t.ent
	if m := c.methodOf(e); m != 0 {
		fn = m
	}
	if c.p.R.Entity(fn).Kind != sem.EntFn || c.p.R.Fn(fn).Body != 0 {
		return c.sharedSinks(fn, c.bindAt(x, in, t))
	}
	var out []sem.EntityID
	params := c.declaredParams(fn)
	for k := range in.Args {
		for _, q := range t.bind(k) {
			if !c.interiorParam(fn, q) || !c.sharedStorage(x.g.Locals[in.Args[k]].Type) {
				continue
			}
			for _, v := range params {
				if v != q && c.keepableParam(v) && !slices.Contains(out, v) {
					out = append(out, v)
				}
			}
		}
	}
	return out
}

// bindAt binds each callable parameter of t's fn that call in passes only fn items and closures x can name.
func (c *borrowCheck) bindAt(x *sinkCtx, in midir.Inst, t sinkTarget) map[int][]sem.EntityID {
	var out map[int][]sem.EntityID
	declared := c.declaredParams(t.ent)
	for k, a := range in.Args {
		for _, q := range t.bind(k) {
			if !c.callableNode(c.p.R.Entity(q).Type) {
				continue
			}
			if ents, ok := c.callablesIn(x, a); ok {
				if out == nil {
					out = map[int][]sem.EntityID{}
				}
				out[slices.Index(declared, q)] = ents
			}
		}
	}
	return out
}

// callablesIn are the fn items and closures a may hold: one closure by its type, else by the flow through x's binding
// of its own parameters; false if some value it may hold is neither.
func (c *borrowCheck) callablesIn(x *sinkCtx, a midir.LocalID) ([]sem.EntityID, bool) {
	if n := c.p.R.Types.Node(x.g.Locals[a].Type); n.Kind == sem.KClosure && n.Ent != 0 && c.p.R.Entity(n.Ent).Kind == sem.EntClosure {
		return []sem.EntityID{n.Ent}, true
	}
	var out []sem.EntityID
	declared := c.declaredParams(x.g.Ent)
	for f := range x.fs[a] {
		e := sem.EntityID(f)
		switch {
		case f == opaqueFact || f&offBit != 0:
			return nil, false
		case c.p.R.Entity(e).Kind == sem.EntFn || c.p.R.Entity(e).Kind == sem.EntClosure:
			out = append(out, e)
		default:
			b, ok := x.bind[slices.Index(declared, e)]
			if !ok {
				return nil, false
			}
			out = append(out, b...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out), len(x.fs[a]) > 0
}

// unknownStores is every operand of a call to an unknown closure or interface method, which keeps storage of its own;
// else each operand but a plain fn callee and one that alone may be shared storage (by its type or a type parameter).
func (c *borrowCheck) unknownStores(x *sinkCtx, in midir.Inst) []sharedStore {
	var cells []int
	for k, a := range in.Args {
		if k == 0 && in.Op != midir.OpCall {
			continue
		}
		if t := x.g.Locals[a].Type; c.sharedStorage(t) || c.facts().NamesTypeParam(t) {
			cells = append(cells, k)
		}
	}
	whole := c.capturingCallee(x.g, in) || in.Ent != 0 && c.ifaceRequirement(in.Ent)
	if !whole && len(cells) == 0 {
		return nil
	}
	var out []sharedStore
	for k := range in.Args {
		plainFn := k == 0 && in.Op != midir.OpCall
		if whole || !plainFn && (len(cells) > 1 || cells[0] != k) {
			out = append(out, sharedStore{arg: k})
		}
	}
	return out
}

// capturingCallee reports a value call whose callee may be a closure, which can keep what it is given in a capture.
func (c *borrowCheck) capturingCallee(g *midir.Func, in midir.Inst) bool {
	t := g.Locals[in.Args[0]].Type
	return in.Op != midir.OpCall && c.p.R.Types.Kind(t) != sem.KFn && c.callableNode(t)
}
