package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// taskInst is a use in caller (a call or a fn value) that binds every type parameter of the callee to inst.
type taskInst struct {
	caller EntityID
	inst   []TypeID
}

// taskUse is a use of generic fn at n; recv is the receiver a method of a generic type takes its type arguments from.
type taskUse struct {
	n    syntax.NodeID
	fn   EntityID
	recv TypeID
}

func (c *checker) noteTaskUse(n syntax.NodeID, fn EntityID, recv TypeID) {
	if c.r.Entities[fn].Kind == EntFn && len(c.r.Fn(fn).TypeParams) > 0 {
		c.taskUses = append(c.taskUses, taskUse{n, fn, recv})
	}
}

// noteTaskUses records the bindings of the uses noted since the last full expression; a parameter nothing binds
// stays open.
func (c *checker) noteTaskUses() {
	for _, u := range c.taskUses {
		var own []TypeID
		for _, in := range c.r.Instances[u.fn] {
			if in.Node == u.n && in.File == c.f {
				own = in.Args
			}
		}
		own = slices.Clone(own)
		for i, t := range own {
			own[i] = c.vars.resolve(t)
		}
		subst := c.callSubst(u.fn, own, c.vars.resolve(u.recv))
		var inst []TypeID
		for _, p := range c.r.Fn(u.fn).TypeParams {
			t, ok := subst[p]
			if !ok {
				t = c.r.Types.Param(p)
			}
			inst = append(inst, t)
		}
		if c.r.taskInsts == nil {
			c.r.taskInsts = map[EntityID][]taskInst{}
		}
		c.r.taskInsts[u.fn] = append(c.r.taskInsts[u.fn], taskInst{c.r.bodyOwner(c.fn), inst})
	}
	c.taskUses = c.taskUses[:0]
}

// instSubsts are the bindings of g's type parameters its callers make, followed through generic callers up to depth;
// cut says some were left out.
func (r *Result) instSubsts(g EntityID, depth int) (out []map[EntityID]TypeID, cut bool) {
	if r.Entities[g].Kind != EntFn {
		return nil, false
	}
	params := r.Fn(g).TypeParams
	seen := map[string]bool{}
	add := func(s map[EntityID]TypeID) {
		key := ""
		for _, p := range params {
			key += r.TypeString(s[p]) + ";"
		}
		switch {
		case seen[key]:
		case len(out) == 64:
			cut = true
		default:
			seen[key] = true
			out = append(out, s)
		}
	}
	for _, e := range r.taskInsts[g] {
		if len(e.inst) != len(params) {
			cut = true
			continue
		}
		s := instSubst(params, e.inst)
		if depth == 0 || !slices.ContainsFunc(e.inst, r.OpenType) || e.caller == g {
			add(s)
			continue
		}
		outer, more := r.instSubsts(e.caller, depth-1)
		cut = cut || more
		if len(outer) == 0 {
			add(s)
		}
		for _, cs := range outer {
			composed := map[EntityID]TypeID{}
			for k, v := range s {
				composed[k] = r.Types.Subst(v, cs)
			}
			add(composed)
		}
	}
	return out, cut
}
