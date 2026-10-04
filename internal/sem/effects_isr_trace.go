package sem

import "kigumi/internal/syntax"

type isrHop struct {
	file FileID
	node syntax.NodeID
	text string
}

// isrFinding is isrTrace's verdict: the code to report, plus the type cInterruptDrop's message alone needs.
type isrFinding struct {
	code Code
	drop TypeID
}

// reportIsrPath re-walks inferIsrSafe's unsafe edges, picking one concrete witness path for the diagnostic's notes.
func (r *Result) reportIsrPath(handler EntityID, safe map[EntityID]bool) {
	e := &r.Entities[handler]
	find := isrFinding{code: cInterruptUnsafeExtern}
	hops := r.isrTrace(handler, safe, map[EntityID]bool{handler: true}, &find)
	args := []any{e.Name}
	if find.code == cInterruptDrop {
		args = append(args, r.TypeString(find.drop))
	}
	d, ok := r.diagAt(e.File, e.Node, find.code, args...)
	if !ok {
		return
	}
	for _, h := range hops {
		d = d.WithNote(r.loc(h.file, r.Files[h.file].span(h.node)), h.text)
	}
	r.emitDiag(e.File, d)
}

// isrTrace picks one witness path through id's edges; a cycle is held as a fallback so a later, more specific edge still wins.
func (r *Result) isrTrace(id EntityID, safe map[EntityID]bool, visited map[EntityID]bool, find *isrFinding) []isrHop {
	var cycle []isrHop
	var cycleFind isrFinding
	for _, e := range r.edgesOf(id) {
		switch e.Kind {
		case EdgeAlloc:
			find.code = cInterruptAlloc
			return []isrHop{{e.File, e.Node, "allocates or deallocates here"}}
		case EdgePanic:
			find.code = cInterruptPanic
			return []isrHop{{e.File, e.Node, "may panic here"}}
		case EdgeIO:
			find.code = cInterruptUnsafeExtern
			return []isrHop{{e.File, e.Node, "performs I/O here"}}
		case EdgeSuspend:
			find.code = cInterruptUnsafeExtern
			return []isrHop{{e.File, e.Node, "may suspend here"}}
		case EdgeWitness, EdgeParamCall, EdgeWitnessCall:
			find.code = cInterruptUnknown
			return []isrHop{{e.File, e.Node, "dispatches dynamically; the target is not known until run time"}}
		case EdgeCallback:
			if e.Target == 0 {
				find.code = cInterruptUnknown
				return []isrHop{{e.File, e.Node, "calls a function value whose target is not known until run time"}}
			}
			if hops, isCycle := r.isrTraceCall(e.Target, e.File, e.Node, safe, visited, find); hops != nil {
				if !isCycle {
					return hops
				}
				if cycle == nil {
					cycle, cycleFind = hops, *find
				}
			}
		case EdgeCall:
			if hops, isCycle := r.isrTraceCall(e.Target, e.File, e.Node, safe, visited, find); hops != nil {
				if !isCycle {
					return hops
				}
				if cycle == nil {
					cycle, cycleFind = hops, *find
				}
			}
		case EdgeDrop:
			if !r.isrDropSafe(e.Type, safe) {
				return r.isrTraceDrop(e.Type, e.File, e.Node, safe, visited, find)
			}
		}
	}
	if cycle != nil {
		*find = cycleFind
		return cycle
	}
	return nil
}

// isrTraceCall traces one call edge, also reporting whether the result is only a cycle back onto the path.
func (r *Result) isrTraceCall(target EntityID, file FileID, node syntax.NodeID, safe map[EntityID]bool, visited map[EntityID]bool, find *isrFinding) ([]isrHop, bool) {
	if target == 0 || r.isrTargetSafe(target, safe) {
		return nil, false
	}
	name := r.Entities[target].Name
	if r.Entities[target].Kind == EntFn && r.Fn(target).Body == 0 {
		find.code = cInterruptUnsafeExtern
		return []isrHop{{file, node, "calls `" + name + "`, which is not declared `noalloc` and has no body the compiler can check"}}, false
	}
	if visited[target] {
		find.code = cInterruptUnknown
		return []isrHop{{file, node, "calls `" + name + "`, recursing back into a function already on this path"}}, true
	}
	visited[target] = true
	sub := r.isrTrace(target, safe, visited, find)
	delete(visited, target)
	if sub == nil {
		return nil, false
	}
	return append([]isrHop{{file, node, "calls `" + name + "`"}}, sub...), false
}

func (r *Result) isrTraceDrop(t TypeID, file FileID, node syntax.NodeID, safe map[EntityID]bool, visited map[EntityID]bool, find *isrFinding) []isrHop {
	find.code, find.drop = cInterruptDrop, t
	head := isrHop{file, node, "drops `" + r.TypeString(t) + "`"}
	if sub := r.isrTraceDropInto(t, file, node, safe, visited, map[TypeID]bool{}, find); sub != nil {
		return append([]isrHop{head}, sub...)
	}
	return []isrHop{head}
}

// isrTraceDropInto mirrors isrDropSafeMemo's descent, returning the hop chain to the first unsafe part (nil if t drops safely).
func (r *Result) isrTraceDropInto(t TypeID, file FileID, node syntax.NodeID, safe map[EntityID]bool, visited map[EntityID]bool, memo map[TypeID]bool, find *isrFinding) []isrHop {
	if t == 0 {
		return nil
	}
	if memo[t] {
		return nil
	}
	memo[t] = true
	tt := r.Types
	n := tt.Node(t)
	switch n.Kind {
	case KIface:
		return []isrHop{{file, node, "drops a dynamic `" + r.TypeString(t) + "` value, whose concrete drop is not known until run time"}}
	case KParam:
		return []isrHop{{file, node, "drops `" + r.TypeString(t) + "`, an unconstrained type parameter whose concrete drop is not known until run time"}}
	case KClosure:
		if n.Ent == 0 {
			return nil
		}
		for _, cap := range r.closure(n.Ent).Captures {
			if sub := r.isrTraceDropInto(r.Entities[cap.Local].Type, file, node, safe, visited, memo, find); sub != nil {
				return sub
			}
		}
		return nil
	case KNamed:
	default:
		return nil
	}
	info := r.typeDecl(n.Ent)
	if info.Form == FormResource && info.Drop != 0 && !r.isrTargetSafe(info.Drop, safe) {
		hops, _ := r.isrTraceCall(info.Drop, file, node, safe, visited, find)
		return hops
	}
	if r.isSharedCell(n.Ent) {
		return []isrHop{{file, node, "drops a `Shared` value, which may free its contents"}}
	}
	subst := map[EntityID]TypeID{}
	for i, p := range info.Params {
		if i < len(n.Args) {
			subst[p] = n.Args[i]
		}
	}
	for _, a := range n.Args {
		if sub := r.isrTraceDropInto(a, file, node, safe, visited, memo, find); sub != nil {
			return append([]isrHop{{file, node, "drops `" + r.TypeString(a) + "`"}}, sub...)
		}
	}
	for _, f := range info.Fields {
		ft := tt.Subst(r.Entities[f].Type, subst)
		if sub := r.isrTraceDropInto(ft, file, node, safe, visited, memo, find); sub != nil {
			return append([]isrHop{{file, node, "drops `" + r.TypeString(ft) + "`"}}, sub...)
		}
	}
	for _, v := range info.Variants {
		for _, p := range r.variant(v).Payload {
			pt := tt.Subst(p, subst)
			if sub := r.isrTraceDropInto(pt, file, node, safe, visited, memo, find); sub != nil {
				return append([]isrHop{{file, node, "drops `" + r.TypeString(pt) + "`"}}, sub...)
			}
		}
	}
	return nil
}
