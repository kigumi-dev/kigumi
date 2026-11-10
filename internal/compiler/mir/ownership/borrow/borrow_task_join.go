package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// joins is what awaiting v certainly joins; a v not traced to exactly one started Task, parameter or Future body joins
// nothing.
func (r *taskRun) joins(v midir.LocalID, depth int) srcSet {
	if depth > 16 {
		return nil
	}
	defs := r.defs[v]
	if len(defs) == 0 {
		if j := r.slot(v); j >= 0 && j < r.nslot && r.seeds[v].has(j) {
			return srcSet{}.with(j)
		}
		return nil
	}
	if len(defs) != 1 {
		return nil
	}
	d := defs[0]
	switch d.Op {
	case midir.OpCopy, midir.OpMove, midir.OpAlias, midir.OpShare:
		return r.joins(d.Args[0], depth+1)
	case midir.OpField, midir.OpPayload:
		if src := r.builtFrom(d.Args[0], depth); src != nil && d.Index < len(src.Args) {
			return r.joins(src.Args[d.Index], depth+1)
		}
		if d.Op == midir.OpPayload && r.onePayload(r.g.Locals[d.Args[0]].Type) {
			return r.joins(d.Args[0], depth+1)
		}
	case midir.OpVariant:
		if len(d.Args) == 1 {
			return r.joins(d.Args[0], depth+1)
		}
	case midir.OpFutureNew:
		return r.calleeJoins(d, depth)
	case midir.OpCall, midir.OpCallValue:
		if s, ok := r.srcOf[srcKey{d, -1}]; ok {
			return srcSet{}.with(s).or(r.spawnedJoins(d, depth))
		}
	}
	return nil
}

// onePayload reports a variant type with a single payload that can hold a handle, as an option has: the payload is all
// of it.
func (r *taskRun) onePayload(t sem.TypeID) bool {
	n := r.c.p.R.Types.Node(t)
	if n.Kind != sem.KNamed || r.c.p.R.Entity(n.Ent).Kind != sem.EntType {
		return false
	}
	d := r.c.p.R.TypeDecl(n.Ent)
	if d == nil || d.Form != sem.FormAdt {
		return false
	}
	payloads := 0
	for _, v := range d.Variants {
		payloads += len(r.c.p.R.Variant(v).Payload)
	}
	return payloads == 1
}

// builtFrom is the record or variant literal v is a copy of, nil when v may be anything else.
func (r *taskRun) builtFrom(v midir.LocalID, depth int) *midir.Inst {
	for ; depth <= 16; depth++ {
		defs := r.defs[v]
		if len(defs) != 1 {
			return nil
		}
		switch d := defs[0]; d.Op {
		case midir.OpRecord, midir.OpVariant:
			return d
		case midir.OpCopy, midir.OpMove, midir.OpAlias, midir.OpShare:
			v = d.Args[0]
		default:
			return nil
		}
	}
	return nil
}

func (r *taskRun) soleCallee(d *midir.Inst) (taskCallee, bool) {
	cs := r.callees(d)
	if len(cs.known) != 1 || cs.param || cs.unknown {
		return taskCallee{}, false
	}
	return cs.known[0], true
}

func (r *taskRun) calleeJoins(d *midir.Inst, depth int) srcSet {
	ce, ok := r.soleCallee(d)
	if !ok || ce.fate.all {
		return nil
	}
	var out srcSet
	ce.fate.joins.each(func(j int) {
		if k := ce.off + j; k < len(d.Args) && j < ce.fate.nslot {
			out = out.or(r.joins(d.Args[k], depth+1))
		}
	})
	return out
}

// spawnedJoins is what joining a Task a bodiless call started from a Future joins: what that Future's body joins.
func (r *taskRun) spawnedJoins(d *midir.Inst, depth int) srcSet {
	ce, ok := r.soleCallee(d)
	if !ok || r.c.p.ByEnt[ce.ent] != nil {
		return nil
	}
	var out srcSet
	for j, q := range r.c.declaredParams(ce.ent) {
		if k := ce.off + j; k < len(d.Args) && r.c.runsFuture(ce.ent, q) {
			out = out.or(r.joins(d.Args[k], depth+1))
		}
	}
	return out
}
