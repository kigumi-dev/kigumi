package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// holderRes is a holder local's resolved set: every method value it can hold, one per method for the proof, and
// whether it is ever unknown. Members of one alias cycle share it; called and escaped are marked once.
type holderRes struct {
	vals, reps      []*valueInstData
	poison          bool
	called, escaped bool
}

// noteHolderValue records a value a local is bound to anywhere in the body: a method value joins its set, a local
// identifier is an alias (that identifier is then not an escape), anything else is unknown. Order is not consulted.
func (c *checker) noteHolderValue(local EntityID, value syntax.NodeID) {
	u := c.t.Unparen(value)
	switch {
	case c.valueExprs[u] != nil:
		if c.holderVals == nil {
			c.holderVals = map[EntityID][]*valueInstData{}
		}
		c.holderVals[local] = append(c.holderVals[local], c.valueExprs[u])
	case c.t.Kind(u) == syntax.Ident && c.r.Entities[c.info.Uses[u]].Kind == EntLocal:
		if c.holderAlias == nil {
			c.holderAlias = map[EntityID][]EntityID{}
			c.holderAliasNode = map[syntax.NodeID]bool{}
		}
		c.holderAlias[local] = append(c.holderAlias[local], c.info.Uses[u])
		c.holderAliasNode[u] = true
	default:
		if c.holderOther == nil {
			c.holderOther = map[EntityID]bool{}
		}
		c.holderOther[local] = true
	}
}

// resolveHolders resolves each holder once by alias-graph components in dependency order: a component holds its own and
// its aliased components' values, and is unknown if assigned a non-method or aliasing an unknown or empty one.
func (c *checker) resolveHolders() {
	c.holderRes = map[EntityID]*holderRes{}
	var nodes []EntityID
	for v := range c.holderVals {
		nodes = append(nodes, v)
	}
	for v := range c.holderAlias {
		nodes = append(nodes, v)
	}
	for v := range c.holderOther {
		nodes = append(nodes, v)
	}
	slices.Sort(nodes)
	idx, low, on := map[EntityID]int{}, map[EntityID]int{}, map[EntityID]bool{}
	var stack []EntityID
	var strong func(v EntityID)
	strong = func(v EntityID) {
		idx[v], low[v] = len(idx), len(idx)
		stack, on[v] = append(stack, v), true
		for _, w := range c.holderAlias[v] {
			if _, seen := idx[w]; !seen {
				strong(w)
				low[v] = min(low[v], low[w])
			} else if on[w] {
				low[v] = min(low[v], idx[w])
			}
		}
		if low[v] == idx[v] {
			var comp []EntityID
			for {
				w := stack[len(stack)-1]
				stack, on[w] = stack[:len(stack)-1], false
				if comp = append(comp, w); w == v {
					break
				}
			}
			c.resolveComp(comp)
		}
	}
	for _, v := range nodes {
		if _, seen := idx[v]; !seen {
			strong(v)
		}
	}
}

func (c *checker) resolveComp(comp []EntityID) {
	res := &holderRes{}
	val, meth := map[*valueInstData]bool{}, map[EntityID]bool{}
	add := func(d *valueInstData) {
		if !val[d] {
			val[d] = true
			res.vals = append(res.vals, d)
			if !meth[d.m] {
				meth[d.m] = true
				res.reps = append(res.reps, d)
			}
		}
	}
	for _, v := range comp {
		res.poison = res.poison || c.holderOther[v]
		for _, d := range c.holderVals[v] {
			add(d)
		}
		for _, w := range c.holderAlias[v] {
			if slices.Contains(comp, w) {
				continue
			}
			r := c.holderRes[w]
			if r == nil || len(r.vals) == 0 || r.poison {
				res.poison = true
			}
			if r != nil {
				for _, d := range r.vals {
					add(d)
				}
			}
		}
	}
	for _, v := range comp {
		c.holderRes[v] = res
	}
}

func (c *checker) holderOf(ent EntityID) *holderRes {
	if r := c.holderRes[ent]; r != nil && len(r.vals) > 0 {
		return r
	}
	return nil
}
