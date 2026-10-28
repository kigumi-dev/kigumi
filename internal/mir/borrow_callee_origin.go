package mir

type calleeOrigin struct {
	param    int
	body     *Func
	captures []LocalID
	known    bool
}

func (c *borrowCheck) calleeOrigin(g *Func, defs map[LocalID]*Inst, l LocalID) calleeOrigin {
	if i := paramIndex(g, defs, l); i >= 0 {
		return calleeOrigin{param: i + 1}
	}
	d := defs[copiedFrom(defs, l)]
	if d == nil || d.Op != OpClosure && d.Op != OpFnItem && d.Op != OpBind {
		return calleeOrigin{}
	}
	o := calleeOrigin{body: c.p.ByEnt[d.Ent], known: true}
	if d.Op != OpFnItem {
		o.captures = d.Args
	}
	return o
}

func (c *borrowCheck) frameCallee(g *Func, defs map[LocalID]*Inst, l LocalID, depth int) bool {
	if !c.calleeKeepsBorrow(g.Locals[l].Type) {
		return false
	}
	o := c.calleeOrigin(g, defs, l)
	if o.param > 0 || o.body == nil || depth == 0 {
		return true
	}
	for _, s := range c.callableSites(o.body, len(o.captures), siteOut) {
		if s.callee == 0 {
			continue
		}
		cap := s.callee - 1
		if cap < len(o.captures) && c.frameCallee(g, defs, o.captures[cap], depth-1) {
			return true
		}
	}
	return false
}
