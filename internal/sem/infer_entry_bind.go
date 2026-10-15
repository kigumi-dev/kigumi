package sem

import "kigumi/internal/syntax"

func (r *Result) markDeferred(dst EntityID, viaCopy bool) {
	m := &r.flow.directDeferred
	if viaCopy {
		m = &r.flow.copyDeferred
	}
	if *m == nil {
		*m = map[EntityID]bool{}
	}
	(*m)[dst] = true
}

// A field a callee only reads through copies made in its own body is bound to what the caller held when it called.
func (r *Result) entryOnlyField(dst EntityID) bool {
	return r.flow.copyDeferred[dst] && !r.flow.directDeferred[dst]
}

func (r *Result) noteBindSrc(file FileID, node syntax.NodeID, tgt, dst EntityID, src fieldFnKey) {
	if r.flow.bindSrc == nil {
		r.flow.bindSrc = map[edgeSiteKey]fieldFnKey{}
	}
	r.flow.bindSrc[edgeSiteKey{File: file, Node: node, Target: tgt, Param: dst}] = src
}

// pruneEntryBinds drops a binding of a fn the callee itself stored: the callee's copy sees that write through its own merge.
func (r *Result) pruneEntryBinds() {
	for id := range r.Entities {
		if k := r.Entities[id].Kind; k != EntFn && k != EntClosure {
			continue
		}
		edges := r.edgesPtr(EntityID(id))
		kept := (*edges)[:0:0]
		for _, e := range *edges {
			if e.Kind == EdgeFieldBind && r.entryOnlyField(e.Param) && !r.boundAtEntry(e) {
				continue
			}
			kept = append(kept, e)
		}
		if len(kept) != len(*edges) {
			*edges = kept
		}
	}
}

func (r *Result) boundAtEntry(e EffectEdge) bool {
	src, ok := r.flow.bindSrc[edgeSiteKey{File: e.File, Node: e.Node, Target: e.Target, Param: e.Param}]
	if !ok {
		return true
	}
	site, ok := r.flow.order[fieldSiteKey{Key: src, Fn: e.Target}]
	return !ok || r.writeMayPrecede(site, copyCut{file: e.File, node: e.Node, root: r.rootOf(src.Ent)})
}
