package sem

import "kigumi/internal/syntax"

type isrHop struct {
	file FileID
	node syntax.NodeID
	text string
}

// isrFinding is isrTrace's verdict: the code to report, plus the type cInterruptDrop and the operation cInterruptUnpermitted need.
type isrFinding struct {
	code Code
	drop TypeID
	op   string
}

// reportIsrPath re-walks the handler's unsafe edges, picking one concrete witness path for the diagnostic's notes.
func (r *Result) reportIsrPath(sc *isrScope) {
	handler := sc.handler
	e := &r.Entities[handler]
	find := isrFinding{code: cInterruptUnsafeExtern}
	hops := r.isrTrace(handler, sc, map[EntityID]bool{handler: true}, &find)
	args := []any{e.Name}
	switch find.code {
	case cInterruptDrop:
		args = append(args, r.TypeString(find.drop))
	case cInterruptUnpermitted:
		args = append(args, find.op)
	}
	d, ok := r.diagAt(e.File, e.Node, find.code, args...)
	if !ok {
		return
	}
	d = d.WithHelp(sc.configHelp(e.Name))
	for _, h := range hops {
		d = d.WithNote(r.loc(h.file, r.Files[h.file].span(h.node)), h.text)
	}
	r.emitDiag(e.File, d)
}

// isrTrace picks one witness path through id's edges; a cycle is held as a fallback so a later, more specific edge still wins.
func (r *Result) isrTrace(id EntityID, sc *isrScope, visited map[EntityID]bool, find *isrFinding) []isrHop {
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
		case EdgeAsm:
			if hops, done := r.isrTraceAsm(e, sc, visited, find); done {
				return hops
			}
		case EdgeWitness, EdgeParamCall, EdgeWitnessCall:
			if e.Kind == EdgeWitness && e.Target != 0 {
				if hops, isCycle := r.isrTraceCall(e.Target, e.File, e.Node, sc, visited, find); hops != nil && !isCycle {
					return hops
				}
				continue
			}
			find.code = cInterruptUnknown
			return []isrHop{{e.File, e.Node, "dispatches dynamically; the target is not known until run time"}}
		case EdgeCallback:
			if e.Target == 0 {
				find.code = cInterruptUnknown
				return []isrHop{{e.File, e.Node, "calls a function value whose target is not known until run time"}}
			}
			if hops, isCycle := r.isrTraceCall(e.Target, e.File, e.Node, sc, visited, find); hops != nil {
				if !isCycle {
					return hops
				}
				if cycle == nil {
					cycle, cycleFind = hops, *find
				}
			}
		case EdgeCall:
			if hops, isCycle := r.isrTraceCall(e.Target, e.File, e.Node, sc, visited, find); hops != nil {
				if !isCycle {
					return hops
				}
				if cycle == nil {
					cycle, cycleFind = hops, *find
				}
			}
		case EdgeDrop:
			if !r.isrDropSafe(e.Type, sc) {
				return r.isrTraceDrop(e.Type, e.File, e.Node, sc, visited, find)
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
func (r *Result) isrTraceCall(target EntityID, file FileID, node syntax.NodeID, sc *isrScope, visited map[EntityID]bool, find *isrFinding) ([]isrHop, bool) {
	if target == 0 || r.isrTargetSafe(target, sc) {
		return nil, false
	}
	name := r.Entities[target].Name
	if r.Entities[target].Kind == EntFn && r.Fn(target).Body == 0 {
		return r.isrBodilessHop(target, file, node, sc, find), false
	}
	if visited[target] {
		find.code = cInterruptUnknown
		return []isrHop{{file, node, "calls `" + name + "`, recursing back into a function already on this path"}}, true
	}
	visited[target] = true
	sub := r.isrTrace(target, sc, visited, find)
	delete(visited, target)
	if sub == nil {
		return nil, false
	}
	return append([]isrHop{{file, node, "calls `" + name + "`"}}, sub...), false
}
