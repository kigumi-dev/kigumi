package borrow

import (
	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// taskCallee is one body a call may run: its summary under the callables the call binds, and the argument its first
// declared parameter takes.
type taskCallee struct {
	ent  sem.EntityID
	fate *taskFate
	off  int
	run  *taskRun
}

// alone is the callee's summary with its callable parameters left to its callers, as its own check sees it.
func (ce *taskCallee) alone() *taskFate { return ce.run.c.taskFateOf(ce.ent, nil) }

// taskCallees are what a call may run: known bodies, a parameter of this body its callers bind, or something unseen.
type taskCallees struct {
	known   []taskCallee
	param   bool
	unknown bool
}

func (r *taskRun) callees(in *midir.Inst) taskCallees {
	if cs, ok := r.calls[in]; ok {
		return cs
	}
	cs := r.resolveCallees(in)
	r.calls[in] = cs
	return cs
}

func (r *taskRun) resolveCallees(in *midir.Inst) taskCallees {
	c := r.c
	var out taskCallees
	add := func(e sem.EntityID, off int) {
		ce := taskCallee{ent: e, off: off, run: r}
		ce.fate = c.taskFateOf(e, r.bindFor(e, in, off))
		out.known = append(out.known, ce)
	}
	switch {
	case in.Op == midir.OpCall && in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn:
		add(in.Ent, 0)
	case in.Op == midir.OpCallValue && c.ifaceRequirement(in.Ent):
		for _, w := range c.implementers(in.Ent) {
			add(w, 1)
		}
		if len(out.known) == 0 {
			v := r.callablesIn(in.Args[0])
			for _, e := range v.ents {
				add(e, 1)
			}
			out.param, out.unknown = v.param, v.unknown || len(v.ents) == 0 && !v.param
		}
	case in.Op == midir.OpCallValue || in.Op == midir.OpFutureNew:
		v := r.callablesIn(in.Args[0])
		if cl := c.calleeClosure(r.g, *in); cl != 0 {
			v = bindVal{ents: []sem.EntityID{cl}}
		} else if in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn && len(v.ents) == 0 {
			v.ents = []sem.EntityID{in.Ent}
		}
		if v.unknown {
			if ents, param, unseen := r.originCallees(in.Args[0]); !unseen {
				v = bindVal{ents: ents, param: param}
			}
		}
		for _, e := range v.ents {
			off := 1
			if c.methodOf(e) != 0 {
				off = 0
			}
			add(e, off)
		}
		out.param, out.unknown = v.param, v.unknown || len(v.ents) == 0 && !v.param
	default:
		out.unknown = true
	}
	return out
}

// originCallees settles a callee value its facts leave unseen by where it was made: plain fn items read out of an
// array or a record, or a parameter of this body (a witness among them), which its callers bind.
func (r *taskRun) originCallees(l midir.LocalID) (ents []sem.EntityID, param, unseen bool) {
	if r.sole == nil {
		r.sole = mirflow.SoleDefs(r.g)
	}
	origins, ok := r.c.calleeOrigins(r.g, r.sole, l)
	for _, o := range origins {
		switch {
		case o.isParam:
			if j := o.param; r.bind != nil {
				v := r.paramBinding(r.slot(r.g.Params[j]))
				ents, param, unseen = append(ents, v.ents...), param || v.param, unseen || v.unknown
				continue
			}
			param = true
		case o.known && len(o.layers) == 0 && r.c.p.R.Entity(o.ent).Kind == sem.EntFn:
			ents = append(ents, o.ent)
		default:
			return nil, false, true
		}
	}
	return ents, param, unseen || !ok || len(origins) == 0
}

// takesByValue leaves out a shared receiver and a reference, which only lend the argument.
func (c *borrowCheck) takesByValue(e sem.EntityID, j int) bool {
	params := c.declaredParams(e)
	if j < 0 || j >= len(params) {
		return false
	}
	if m := c.methodOf(e); m != 0 {
		e = m
	}
	if c.p.R.Entity(e).Kind == sem.EntFn {
		return c.byValueParam(c.p.R.Fn(e), params[j])
	}
	return c.p.R.Types.Kind(c.p.R.Entity(params[j]).Type) != sem.KRef
}
