package borrow

import (
	"fmt"
	"slices"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// bindVal is what a callable parameter slot of a callee is at one call: the fns and closures it may be, a parameter
// of the calling body that the caller's own callers bind (param), or a value whose origin is lost (unknown).
type bindVal struct {
	ents    []sem.EntityID
	param   bool
	unknown bool
}

func (v bindVal) join(o bindVal) bindVal {
	ents := slices.Clone(v.ents)
	for _, e := range o.ents {
		if !slices.Contains(ents, e) {
			ents = append(ents, e)
		}
	}
	return bindVal{ents: ents, param: v.param || o.param, unknown: v.unknown || o.unknown}
}

// taskBind binds a body's callable parameter slots; nil leaves every one to its callers, as a body checked on its own.
type taskBind map[int]bindVal

func (b taskBind) key() string {
	if b == nil {
		return "-"
	}
	var parts []string
	for j, v := range b {
		ents := slices.Clone(v.ents)
		slices.Sort(ents)
		parts = append(parts, fmt.Sprint(j, ents, v.param, v.unknown))
	}
	slices.Sort(parts)
	return strings.Join(parts, ";")
}

// bindFor binds the callable slots of callee e from the arguments of in, the first declared parameter taking argument
// off, and a closure's captures from its literals in this body.
func (r *taskRun) bindFor(e sem.EntityID, in *midir.Inst, off int) taskBind {
	if r.c.p.ByEnt[e] == nil && r.c.methodOf(e) == 0 {
		return taskBind{}
	}
	out := taskBind{}
	params := r.c.declaredParams(e)
	used := r.c.callsSlots(e)
	for k := max(off, 0); k < len(in.Args); k++ {
		if !used.has(k - off) {
			continue
		}
		if v := r.callablesIn(in.Args[k]); len(v.ents) > 0 || v.param || v.unknown {
			out[k-off] = v
		}
	}
	if r.c.p.R.Entity(e).Kind == sem.EntClosure {
		caps := r.c.p.R.Closure(e).Captures
		for _, blk := range r.g.Blocks {
			for _, lit := range blk.Insts {
				if lit.Op != midir.OpClosure || lit.Ent != e {
					continue
				}
				for w, a := range lit.Args {
					if !used.has(len(params) + w) {
						continue
					}
					if v := r.callablesIn(a); w < len(caps) && (len(v.ents) > 0 || v.param || v.unknown) {
						out[len(params)+w] = out[len(params)+w].join(v)
					}
				}
			}
		}
	}
	return out
}

// paramBinding is what slot j is bound to; a capture of a closure made elsewhere is that maker's, like a parameter.
func (r *taskRun) paramBinding(j int) bindVal {
	if v, ok := r.bind[j]; ok {
		return v
	}
	if r.bind == nil || j >= r.ndecl {
		return bindVal{param: true}
	}
	return bindVal{unknown: true}
}

func (r *taskRun) paramEntBinding(e sem.EntityID) bindVal {
	for _, l := range r.g.Params {
		if r.g.Locals[l].Ent == e {
			if j := r.slot(l); j >= 0 {
				return r.paramBinding(j)
			}
		}
	}
	return bindVal{unknown: true}
}

// callsSlots are the slots of e's body whose value it may call or hand to another call: the only ones a binding
// changes its summary for.
func (c *borrowCheck) callsSlots(e sem.EntityID) srcSet {
	if m := c.methodOf(e); m != 0 {
		e = m
	}
	g := c.p.ByEnt[e]
	if g == nil {
		return nil
	}
	if s, ok := c.state.taskCalls[g]; ok {
		return s
	}
	_, slot := c.keptSlots(g, c.declaredParams(g.Ent))
	var fs flowSets
	var out srcSet
	for _, l := range g.Params {
		j := slot(l)
		if j < 0 || !c.typeRuns(g.Locals[l].Type, runsCallable) {
			continue
		}
		if fs == nil {
			fs = c.bodyFacts(g)
		}
		e := g.Locals[l].Ent
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				if bodyRoles[in.Op] != roleCall {
					continue
				}
				for _, a := range in.Args {
					if a == l || e != 0 && fs[a][uint32(e)] {
						out = out.with(j)
					}
				}
			}
		}
	}
	if c.state.taskCalls == nil {
		c.state.taskCalls = map[*midir.Func]srcSet{}
	}
	c.state.taskCalls[g] = out
	return out
}
