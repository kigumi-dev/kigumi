package sem

import "kigumi/internal/syntax"

func (r *Result) isrTraceDrop(t TypeID, file FileID, node syntax.NodeID, sc *isrScope, visited map[EntityID]bool, find *isrFinding) []isrHop {
	find.code, find.drop = cInterruptDrop, t
	head := isrHop{file, node, "drops `" + r.TypeString(t) + "`"}
	if sub := r.isrTraceDropInto(t, file, node, sc, visited, map[TypeID]bool{}, find); sub != nil {
		return append([]isrHop{head}, sub...)
	}
	return []isrHop{head}
}

// isrTraceDropInto mirrors isrDropSafeMemo's descent, returning the hop chain to the first unsafe part (nil if t drops safely).
func (r *Result) isrTraceDropInto(t TypeID, file FileID, node syntax.NodeID, sc *isrScope, visited map[EntityID]bool, memo map[TypeID]bool, find *isrFinding) []isrHop {
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
			if sub := r.isrTraceDropInto(r.Entities[cap.Local].Type, file, node, sc, visited, memo, find); sub != nil {
				return sub
			}
		}
		return nil
	case KNamed:
	default:
		return nil
	}
	info := r.typeDecl(n.Ent)
	if info.Form == FormResource && info.Drop != 0 && !r.isrTargetSafe(info.Drop, sc) {
		hops, _ := r.isrTraceCall(info.Drop, file, node, sc, visited, find)
		return hops
	}
	if r.isHeapCell(n.Ent) {
		return []isrHop{{file, node, "drops a `" + r.Entities[n.Ent].Name + "` value, which may free its contents"}}
	}
	subst := map[EntityID]TypeID{}
	for i, p := range info.Params {
		if i < len(n.Args) {
			subst[p] = n.Args[i]
		}
	}
	for _, a := range n.Args {
		if sub := r.isrTraceDropInto(a, file, node, sc, visited, memo, find); sub != nil {
			return append([]isrHop{{file, node, "drops `" + r.TypeString(a) + "`"}}, sub...)
		}
	}
	for _, f := range info.Fields {
		ft := tt.Subst(r.Entities[f].Type, subst)
		if sub := r.isrTraceDropInto(ft, file, node, sc, visited, memo, find); sub != nil {
			return append([]isrHop{{file, node, "drops `" + r.TypeString(ft) + "`"}}, sub...)
		}
	}
	for _, v := range info.Variants {
		for _, p := range r.variant(v).Payload {
			pt := tt.Subst(p, subst)
			if sub := r.isrTraceDropInto(pt, file, node, sc, visited, memo, find); sub != nil {
				return append([]isrHop{{file, node, "drops `" + r.TypeString(pt) + "`"}}, sub...)
			}
		}
	}
	return nil
}
