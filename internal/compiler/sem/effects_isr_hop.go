package sem

import "kigumi/internal/compiler/syntax"

// isrBodilessHop names why a bodiless callee fails: no reviewed contract, an unpermitted contract, or a contracted fact.
func (r *Result) isrBodilessHop(target EntityID, file FileID, node syntax.NodeID, sc *isrScope, find *isrFinding) []isrHop {
	name := r.Entities[target].Name
	s, _ := r.opFixedSummary(target)
	if s.Untrusted {
		find.code = cInterruptUnsafeExtern
		return []isrHop{{file, node, "calls `" + name + "`, which has no body the compiler can check and no reviewed interrupt contract"}}
	}
	for _, id := range splitContracts(s.Contracts) {
		if !sc.permits[id] {
			find.code, find.op = cInterruptUnpermitted, id
			return append([]isrHop{{file, node, "calls `" + name + "`, which performs `" + id + "`"}}, r.isrNearMiss(id, file, node, sc)...)
		}
	}
	find.code = cInterruptUnsafeExtern
	for _, f := range isrFacts {
		if s.Facts[f].Maybe {
			switch f {
			case FactAlloc, FactRelease:
				find.code = cInterruptAlloc
			case FactPanic:
				find.code = cInterruptPanic
			}
			return []isrHop{{file, node, "calls `" + name + "`, which may " + f.String()}}
		}
	}
	return []isrHop{{file, node, "calls `" + name + "`, whose operations no interrupt contract covers"}}
}

// isrNearMiss points at a permission for the same operation that differs in width, implementation or nesting.
func (r *Result) isrNearMiss(id string, file FileID, node syntax.NodeID, sc *isrScope) []isrHop {
	for _, p := range sc.list {
		if isrOpName(p) == isrOpName(id) {
			return []isrHop{{file, node, "the configuration permits `" + p + "`, which differs in width, implementation or nesting condition"}}
		}
	}
	return nil
}

// isrTraceAsm reports an unverifiable asm template, or traces a `sym` operand like a call.
func (r *Result) isrTraceAsm(e EffectEdge, sc *isrScope, visited map[EntityID]bool, find *isrFinding) ([]isrHop, bool) {
	if e.Param == 0 {
		why := r.isrAsmVerdict(e)
		if why == "" {
			return nil, false
		}
		find.code = cInterruptAsm
		return []isrHop{{e.File, e.Node, why}}, true
	}
	if r.asmSymBoxes(e.Param) {
		find.code = cInterruptAlloc
		return []isrHop{{e.File, e.Node, "names `" + r.Entities[e.Param].Name + "`, whose export(C) wrapper may box its arguments"}}, true
	}
	hops, isCycle := r.isrTraceCall(e.Param, e.File, e.Node, sc, visited, find)
	return hops, hops != nil && !isCycle
}
