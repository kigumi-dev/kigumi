package sem

import "kigumi/internal/syntax"

// protoSite is a protocol call whose operand type was still open; a direct type makes the call free.
type protoSite struct {
	edges *[]EffectEdge
	from  int
	nodes []syntax.NodeID
	typ   TypeID
	proto string
}

func (c *checker) noteProtoSite(from int, call syntax.NodeID, typ TypeID, proto string, args ...syntax.NodeID) {
	nodes := []syntax.NodeID{call}
	for _, a := range args {
		nodes = append(nodes, a)
		if c.t.Kind(a) == syntax.BorrowExpr {
			nodes = append(nodes, syntax.NodeID(c.t.Nodes[a].Rhs))
		}
	}
	c.protoSites = append(c.protoSites, protoSite{edges: c.edges, from: from, nodes: nodes, typ: typ, proto: proto})
}

func (c *checker) settleProtoSites() {
	kept := c.protoSites[:0]
	for _, s := range c.protoSites {
		t := c.vars.resolve(s.typ)
		if c.r.Types.ContainsVar(t) {
			kept = append(kept, s)
			continue
		}
		if c.r.ProtocolDirect(t, s.proto) {
			s.elide(c.r)
		}
	}
	c.protoSites = kept
}

func (s protoSite) elide(r *Result) {
	edges := *s.edges
	for i := s.from; i < len(edges); i++ {
		e := &edges[i]
		switch {
		case e.Kind == EdgeCall && e.Node == s.nodes[0]:
			e.Kind = EdgeKnownCall
		case e.Kind == EdgeAlloc && directCrossWhy(e.Why) && s.has(e.Node):
			e.Type = 0
		}
	}
}

func (s protoSite) has(n syntax.NodeID) bool {
	for _, m := range s.nodes {
		if m == n {
			return true
		}
	}
	return false
}
