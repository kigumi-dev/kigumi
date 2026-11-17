package borrow

import (
	"maps"
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

type valueOriginKey struct {
	node  syntax.NodeID
	local midir.LocalID
}

func (c *borrowCheck) flowOrigin(fl storeFlow, local midir.LocalID) (midir.LocalID, bool) {
	if fl.node == 0 || !fl.byValue {
		return c.heldOrigin(local)
	}
	if slices.Contains(c.f.Params, local) {
		return local, true
	}
	if c.originsAt == nil {
		c.collectOriginsAt()
	}
	for _, p := range c.f.Params {
		if c.originsAt[valueOriginKey{node: fl.node, local: local}][uint32(p)] {
			return p, true
		}
	}
	return 0, false
}

func (c *borrowCheck) collectOriginsAt() {
	c.originsAt = map[valueOriginKey]map[uint32]bool{}
	keeps := func(l midir.LocalID) bool {
		t := c.f.Locals[l].Type
		return c.keepsMutRef(t) || c.facts().NamesTypeParam(t)
	}
	entry := flowSets{}
	for _, p := range c.f.Params {
		t := c.f.Locals[p].Type
		if c.keepsMutRef(t) || c.p.R.Types.Kind(t) != sem.KFn && c.facts().NamesTypeParam(t) {
			entry[p] = map[uint32]bool{uint32(p): true}
		}
	}
	if len(entry) == 0 {
		return
	}
	ins := make([]flowSets, len(c.f.Blocks))
	for i := range ins {
		ins[i] = flowSets{}
	}
	ins[0] = entry
	defs := c.soleDefMap()
	edges := make([][]originEdge, len(c.f.Blocks))
	for bid, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			edges[bid] = append(edges[bid], c.originEdge(in, defs, keeps, true))
		}
	}
	order := mirflow.ReversePostorder(c.f)
	for changed := true; changed; {
		changed = false
		for _, bid := range order {
			s := cloneFlowSets(ins[bid])
			for i, in := range c.f.Blocks[bid].Insts {
				if in.Op == midir.OpDrop || in.Op == midir.OpRelease || in.Op == midir.OpCall || in.Op == midir.OpCallValue || in.Op == midir.OpFutureNew {
					for local, origins := range s {
						key := valueOriginKey{node: in.Node, local: local}
						if c.originsAt[key] == nil {
							c.originsAt[key] = map[uint32]bool{}
						}
						maps.Copy(c.originsAt[key], origins)
					}
				}
				e := edges[bid][i]
				sources := map[uint32]bool{}
				for _, a := range e.srcs {
					maps.Copy(sources, s[a])
				}
				if in.Op.Yields() {
					delete(s, in.Dst)
				}
				for _, d := range e.dsts {
					if !keeps(d) {
						continue
					}
					if s[d] == nil {
						s[d] = map[uint32]bool{}
					}
					maps.Copy(s[d], sources)
				}
			}
			for _, target := range c.f.Blocks[bid].Term.Targets {
				changed = mergeFlowSets(ins[target], s) || changed
			}
		}
	}
}
