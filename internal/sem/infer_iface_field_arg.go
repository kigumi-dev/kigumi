package sem

import "kigumi/internal/syntax"

// ifaceFieldArgEdges is knownFieldCallEdges' counterpart for an interface-typed
// field reached through any number of intermediate record fields.
func (c *checker) ifaceFieldArgEdges(n syntax.NodeID, fn EntityID, args []syntax.NodeID) {
	info := c.r.Fn(fn)
	for i, param := range info.Params {
		if i >= len(args) || c.t.Kind(args[i]) == syntax.Spread {
			continue
		}
		for _, path := range c.r.ifaceFieldPathsOf(c.r.Entities[param].Type, nil) {
			dst := param
			for _, f := range path {
				dst = c.r.fieldParamEnt(fieldFnKey{Ent: dst, Field: f})
			}
			c.ifaceFieldArgEdge(n, args[i], path, dst)
		}
	}
}

// ifaceFieldPathsOf lists every field path from t down to an interface-typed
// leaf through intermediate named records; visited bounds a recursive-type cycle.
func (r *Result) ifaceFieldPathsOf(t TypeID, visited map[TypeID]bool) [][]EntityID {
	if r.Types.Kind(t) == KRef {
		t = r.Types.Node(t).Elem
	}
	if r.Types.Kind(t) != KNamed || visited[t] {
		return nil
	}
	if visited == nil {
		visited = map[TypeID]bool{}
	}
	visited[t] = true
	var out [][]EntityID
	for _, f := range r.typeDecl(r.Types.Node(t).Ent).Fields {
		ft := r.Entities[f].Type
		switch r.Types.Kind(ft) {
		case KIface:
			out = append(out, []EntityID{f})
		case KNamed, KRef:
			for _, sub := range r.ifaceFieldPathsOf(ft, visited) {
				out = append(out, append([]EntityID{f}, sub...))
			}
		}
	}
	return out
}

// ifaceFieldArgEdge binds dst to the chain read off arg: a record literal's
// field init recurses one hop; anything else chains fieldChainEnt's root.
func (c *checker) ifaceFieldArgEdge(n, arg syntax.NodeID, path []EntityID, dst EntityID) {
	if len(path) == 0 {
		return
	}
	if c.t.Kind(arg) == syntax.RecordLit {
		for _, entry := range c.t.Children(syntax.NodeID(c.t.Nodes[arg].Rhs)) {
			if c.t.Kind(entry) == syntax.Spread || c.info.Uses[entry] != path[0] {
				continue
			}
			init := syntax.NodeID(c.t.Nodes[entry].Lhs)
			if len(path) == 1 {
				if src := c.fieldChainEnt(init); src != 0 {
					c.edge(EffectEdge{Kind: EdgeIfaceArg, Target: src, Param: dst, Node: n})
				}
				return
			}
			c.ifaceFieldArgEdge(n, init, path[1:], dst)
			return
		}
		return
	}
	src := c.fieldChainEnt(arg)
	for _, f := range path {
		if src == 0 {
			return
		}
		src = c.r.fieldKeyEnt(src, f)
	}
	if src == 0 {
		return
	}
	c.edge(EffectEdge{Kind: EdgeIfaceArg, Target: src, Param: dst, Node: n})
}
