package sem

import "kigumi/internal/syntax"

// OpHop is one step of ExplainOpFact's first-cause chain.
type OpHop struct {
	File FileID
	Node syntax.NodeID
	Text string
}

// OpExplain is ExplainOpFact's answer: Maybe mirrors OpSummary.Facts[f], and
// Chain is empty exactly when Maybe is false.
type OpExplain struct {
	Maybe bool
	Chain []OpHop
}

// ExplainOpFact walks id's first cause for fact f, one hop per call/drop
// edge, stopping once an entity recurs back onto its own path.
func (r *Result) ExplainOpFact(id EntityID, f OpFact) OpExplain {
	s := r.opSummary(id)
	st := s.Facts[f]
	if !st.Maybe {
		return OpExplain{}
	}
	return OpExplain{Maybe: true, Chain: r.opHops(st.Cause, f, map[EntityID]bool{id: true})}
}

func (r *Result) opHops(cause OpCause, f OpFact, visited map[EntityID]bool) []OpHop {
	e := cause.Edge
	if cause.Unknown {
		return []OpHop{{e.File, e.Node, "reaches an unverified or dynamic operation (" + f.String() + ")"}}
	}
	switch e.Kind {
	case EdgeAlloc, EdgePanic, EdgeIO, EdgeSuspend:
		if e.Kind == EdgeAlloc && copyWhy(e.Why) {
			return []OpHop{{e.File, e.Node, r.copyHop(e)}}
		}
		return []OpHop{{e.File, e.Node, "may " + f.String() + " here"}}
	case EdgeDrop:
		return []OpHop{{e.File, e.Node, "drops `" + r.TypeString(e.Type) + "`, which may " + f.String()}}
	case EdgeCall, EdgeWitness, EdgeCallback:
		if e.Target == 0 || visited[e.Target] {
			return []OpHop{{e.File, e.Node, "calls a target whose own cause recurses back onto this path"}}
		}
		head := OpHop{e.File, e.Node, "calls `" + r.Entities[e.Target].Name + "`"}
		visited[e.Target] = true
		sub := r.opSummary(e.Target).Facts[f]
		if !sub.Maybe {
			return []OpHop{head}
		}
		return append([]OpHop{head}, r.opHops(sub.Cause, f, visited)...)
	}
	return nil
}
