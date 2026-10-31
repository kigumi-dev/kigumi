package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

func (c *checker) recordIfaceResult(body syntax.NodeID) {
	delete(c.r.ifaceResult, c.fn)
	if c.fn == 0 || c.r.Fn(c.fn).Declared&EffAsync != 0 || c.r.Types.Kind(c.vars.resolve(c.retType)) != KIface {
		return
	}
	var all []EntityID
	leaves := c.resultLeaves(body)
	for _, leaf := range leaves {
		w, ok := c.info.CoerceWitness[leaf]
		if !ok {
			return
		}
		for _, id := range w {
			if !slices.Contains(all, id) {
				all = append(all, id)
			}
		}
	}
	if len(leaves) > 0 {
		c.r.ifaceResult[c.fn] = all
	}
}

func (c *checker) noteIfaceRecvFn(n, base syntax.NodeID) {
	call, ok := c.info.Calls[c.t.Unparen(base)]
	if !ok || call.Callee == 0 || c.r.Entities[call.Callee].Kind != EntFn || c.r.Fn(call.Callee).Body == 0 {
		return
	}
	c.r.ifaceRecvFn[literalEntKey{File: c.f, Node: n}] = call.Callee
}
