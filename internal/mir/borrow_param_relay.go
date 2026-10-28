package mir

import "slices"

// paramIndex is the parameter of g that l is a copy of, or -1.
func paramIndex(g *Func, defs map[LocalID]*Inst, l LocalID) int {
	l = copiedFrom(defs, l)
	return slices.Index(g.Params, l)
}

func copiedFrom(defs map[LocalID]*Inst, l LocalID) LocalID {
	for range 8 {
		d := defs[l]
		if d == nil || len(d.Args) == 0 || d.Op != OpCopy && d.Op != OpMove && d.Op != OpAlias {
			break
		}
		l = d.Args[0]
	}
	return l
}

func (c *borrowCheck) relayCallee(g *Func, defs map[LocalID]*Inst, l LocalID, kind siteKind) (int, bool) {
	l = copiedFrom(defs, l)
	if i := slices.Index(g.Params, l); i >= 0 {
		return i + 1, false
	}
	d := defs[l]
	if d == nil || d.Op != OpClosure && d.Op != OpFnItem && d.Op != OpBind {
		return 0, false
	}
	h := c.p.ByEnt[d.Ent]
	if h == nil {
		return 0, false
	}
	skip := 0
	if d.Op != OpFnItem {
		skip = len(d.Args)
	}
	callee := 0
	for _, s := range c.callableSites(h, skip, kind) {
		cap := s.callee - 1
		if cap < 0 || cap >= skip {
			return 0, false
		}
		p := paramIndex(g, defs, d.Args[cap]) + 1
		if p == 0 || callee != 0 && callee != p {
			return 0, false
		}
		callee = p
	}
	return callee, callee == 0
}

func (c *borrowCheck) callableSites(h *Func, skip int, kind siteKind) []paramStore {
	if h == nil || skip > len(h.Params) {
		return wholeParam
	}
	var out []paramStore
	for j := skip; j < len(h.Params); j++ {
		if kind == siteStores && len(c.paramSelfSources(h, j)) > 0 {
			return wholeParam
		}
		out = append(out, c.paramSites(paramKey{g: h, i: j, kind: kind})...)
	}
	return out
}

// closureFed keeps, of the values a call may hand to the closure in slot i, those the callee passes on to a parameter
// of the closure that its body may put somewhere; all of them when the callee's use of the closure is not plain calls.
func (c *borrowCheck) closureFed(callee *Func, i int, lit *Inst, values []flowArg) []flowArg {
	h := c.p.ByEnt[lit.Ent]
	if callee == nil || h == nil {
		return values
	}
	feeds, ok := c.callableFeeds(callee, i)
	if !ok {
		return values
	}
	return slices.DeleteFunc(slices.Clone(values), func(v flowArg) bool {
		for p, params := range feeds {
			if slices.Contains(params, v.param) && c.paramOut(h, len(lit.Args)+p) {
				return false
			}
		}
		return true
	})
}

// callableFeeds lists, for each argument position of the calls g makes through its parameter f, the parameters of g
// passed there; false when f is used otherwise or an argument does not come from a parameter.
func (c *borrowCheck) callableFeeds(g *Func, f int) ([][]int, bool) {
	if f >= len(g.Params) {
		return nil, false
	}
	defs := soleDefs(g)
	var feeds [][]int
	for _, blk := range g.Blocks {
		if slices.ContainsFunc(blk.Term.Args, func(a LocalID) bool { return paramIndex(g, defs, a) == f }) {
			return nil, false
		}
		for _, in := range blk.Insts {
			uses := slices.ContainsFunc(in.Args, func(a LocalID) bool { return paramIndex(g, defs, a) == f })
			switch {
			case !uses || in.Op == OpCopy || in.Op == OpMove || in.Op == OpAlias || in.Op == OpDrop || in.Op == OpRelease:
			case in.Op == OpCallValue && paramIndex(g, defs, in.Args[0]) == f && !slices.ContainsFunc(in.Args[1:], func(a LocalID) bool { return paramIndex(g, defs, a) == f }):
				for p, a := range in.Args[1:] {
					j := paramIndex(g, defs, a)
					if j < 0 {
						return nil, false
					}
					for len(feeds) <= p {
						feeds = append(feeds, nil)
					}
					feeds[p] = append(feeds[p], j)
				}
			default:
				return nil, false
			}
		}
	}
	return feeds, true
}
