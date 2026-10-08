package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// EdgeAlloc whys the MIR adds after checking: a copy that ends sharing with another name, and one that keeps a local's value.
const (
	whyCopy     = 8
	whyCopyHeld = 9
	whyCopyElem = 10
)

type CopySite struct {
	Owner EntityID
	File  FileID
	Node  syntax.NodeID
	Type  TypeID
	Held  bool
	Elem  bool
}

func (r *Result) ChargeCopies(sites []CopySite) {
	if len(sites) == 0 {
		return
	}
	nodes := r.effectNodes()
	before := r.noallocVerdicts(nodes)
	handlers := r.interruptHandlers()
	var safe map[EntityID]bool
	if len(handlers) > 0 {
		safe = r.inferIsrSafe(nodes)
	}
	added := false
	for _, s := range sites {
		e := EffectEdge{Kind: EdgeAlloc, Type: s.Type, File: s.File, Node: s.Node, Why: whyCopy}
		switch {
		case s.Held:
			e.Why = whyCopyHeld
		case s.Elem:
			e.Why = whyCopyElem
		}
		if _, node := before[s.Owner]; !node || slices.Contains(r.edgesOf(s.Owner), e) {
			continue
		}
		edges := r.edgesPtr(s.Owner)
		*edges, added = append(*edges, e), true
	}
	if !added {
		return
	}
	r.settleNoalloc(nodes)
	for _, id := range nodes {
		r.reportNoallocBreaks(id, before[id])
	}
	r.inferOpSummaries(nodes)
	if len(handlers) > 0 {
		now := r.inferIsrSafe(nodes)
		for _, id := range handlers {
			if safe[id] && !now[id] && !r.Fn(id).Naked {
				r.reportIsrPath(id, now)
			}
		}
	}
	r.emit()
}

// settleNoalloc lowers the inferred answers from the last fixpoint, which still bounds them: edges were only added.
func (r *Result) settleNoalloc(nodes []EntityID) {
	for changed := true; changed; {
		changed = false
		for _, id := range nodes {
			s := r.summaryPtr(id)
			if _, fixed := r.fixedSummary(id); fixed || !s.Noalloc {
				continue
			}
			for _, e := range r.edgesOf(id) {
				if !r.edgeAllows(e, EffNoalloc) {
					s.Noalloc, changed = false, true
					break
				}
			}
		}
	}
}

func (r *Result) noallocVerdicts(nodes []EntityID) map[EntityID][]bool {
	out := make(map[EntityID][]bool, len(nodes))
	for _, id := range nodes {
		edges := r.edgesOf(id)
		kept := make([]bool, len(edges))
		for i, e := range edges {
			kept[i] = r.keepsNoalloc(id, e)
		}
		out[id] = kept
	}
	return out
}

func (r *Result) keepsNoalloc(id EntityID, e EffectEdge) bool {
	if e.Kind == EdgeContract {
		return e.Mods&EffNoalloc == 0 || allows(r.summary(e.Target), EffNoalloc)
	}
	return r.Entities[id].Kind == EntClosure || r.Fn(id).Declared&EffNoalloc == 0 || r.edgeAllows(e, EffNoalloc)
}

// An edge added after the effect pass was never checked, so only a break the charge caused is reported.
func (r *Result) reportNoallocBreaks(id EntityID, before []bool) {
	for i, e := range r.edgesOf(id) {
		if i < len(before) && !before[i] || r.keepsNoalloc(id, e) {
			continue
		}
		if e.Kind != EdgeContract {
			r.reportEdge(e, EffNoalloc)
			continue
		}
		why := r.violationText(e.Target, EffNoalloc)
		if r.Entities[e.Target].Kind == EntClosure {
			r.errAt(e.File, e.Node, cClosureContract, modName(EffNoalloc), why)
		} else {
			r.errAt(e.File, e.Node, cFnItemContract, r.Entities[e.Target].Name, modName(EffNoalloc), why)
		}
	}
}

func copyWhy(why uint8) bool { return why == whyCopy || why == whyCopyHeld || why == whyCopyElem }

func (r *Result) copyHop(e EffectEdge) string {
	text := "copies `" + r.TypeString(e.Type) + "` into a box of its own here"
	switch e.Why {
	case whyCopyHeld:
		return text + ", because the local keeps its own value"
	case whyCopyElem:
		return text + ", because the element is handed out as a copy"
	}
	return text + ", so it stops sharing a box with another name"
}
