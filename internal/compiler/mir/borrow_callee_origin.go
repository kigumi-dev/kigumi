package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

type calleeOrigin struct {
	param    int
	isParam  bool
	body     *midir.Func
	captures []midir.LocalID
	known    bool
}

func (c *borrowCheck) calleeOrigin(g *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID) calleeOrigin {
	if i := paramIndex(g, defs, l); i >= 0 {
		return calleeOrigin{param: i, isParam: true}
	}
	d := defs[copiedFrom(defs, l)]
	if d == nil || d.Op != midir.OpClosure && d.Op != midir.OpFnItem && d.Op != midir.OpBind {
		return calleeOrigin{}
	}
	o := calleeOrigin{body: c.p.ByEnt[d.Ent], known: true}
	if d.Op != midir.OpFnItem {
		o.captures = d.Args
	}
	return o
}

func (c *borrowCheck) frameCallee(g *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID, depth int) bool {
	if !c.calleeKeepsBorrow(g.Locals[l].Type) {
		return false
	}
	o := c.calleeOrigin(g, defs, l)
	if o.isParam || o.body == nil || depth == 0 {
		return true
	}
	for _, s := range c.callableSites(o.body, len(o.captures), siteOut) {
		if c.frameTarget(g, defs, s.callee, o.captures, depth-1) {
			return true
		}
	}
	return false
}
