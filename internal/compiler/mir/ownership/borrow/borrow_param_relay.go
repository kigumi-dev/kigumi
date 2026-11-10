package borrow

import (
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// paramIndex is the parameter of g that l is a copy of, or -1.
func paramIndex(g *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID) int {
	l = copiedFrom(g, defs, l)
	return slices.Index(g.Params, l)
}

// copiedFrom is the local l copies; it stops at a parameter of g, whose definitions in the body are not its whole value.
func copiedFrom(g *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID) midir.LocalID {
	for range 8 {
		if slices.Contains(g.Params, l) {
			break
		}
		d := defs[l]
		if d == nil || len(d.Args) == 0 || d.Op != midir.OpCopy && d.Op != midir.OpMove && d.Op != midir.OpAlias {
			break
		}
		l = d.Args[0]
	}
	return l
}

func (c *borrowCheck) relayCallee(g *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID, kind siteKind) []relayTarget {
	return c.relayTargets(g, defs, l, false, kind, 8)
}

// relayTargets are where calling l, or a callable l holds, hands a value on.
func (c *borrowCheck) relayTargets(g *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID, held bool, kind siteKind, depth int) []relayTarget {
	if kind == siteOut && !held && !c.calleeKeepsBorrow(g.Locals[l].Type) {
		return nil
	}
	origins, ok := c.argOrigins(g, defs, l, held)
	if ok && len(origins) > 0 && origins[0].isParam {
		return []relayTarget{{kind: relayParam, param: origins[0].param, held: origins[0].held}}
	}
	if !ok || depth == 0 {
		return []relayTarget{{kind: relayUnknown}}
	}
	var out []relayTarget
	for _, o := range origins {
		if o.body == nil {
			return []relayTarget{{kind: relayUnknown}}
		}
		for _, s := range c.callableSites(o.body, o.argParams(), kind) {
			for _, mapped := range c.relaySite(g, defs, s, o.boundLocals(), kind, depth-1) {
				if !slices.Contains(out, mapped.callee) {
					out = append(out, mapped.callee)
				}
			}
		}
	}
	return out
}

// callableSites are the sites of h's parameters params, those the calls of a value running h pass.
func (c *borrowCheck) callableSites(h *midir.Func, params []int, kind siteKind) []paramStore {
	if h == nil || slices.ContainsFunc(params, func(j int) bool { return j >= len(h.Params) }) {
		return wholeParam
	}
	var out []paramStore
	for _, j := range params {
		if kind == siteOut && !c.paramKeepsRef(h, j) && !c.paramKeepsBorrow(h.Locals[h.Params[j]].Type) {
			continue
		}
		if kind == siteStores && len(c.paramSelfSources(h, j)) > 0 {
			return []paramStore{{via: true}}
		}
		out = append(out, c.paramSites(paramKey{g: h, i: j, kind: kind})...)
	}
	return out
}

// closureFed keeps, of the values a call may hand to the closure in slot i, those the callee passes on to a parameter
// of the closure that its body may put somewhere; all of them when the callee's use of the closure is not plain calls.
func (c *borrowCheck) closureFed(callee *midir.Func, i int, lit *midir.Inst, values []flowArg) []flowArg {
	h := c.p.ByEnt[lit.Ent]
	if callee == nil || h == nil {
		return values
	}
	feeds, ok := c.callableFeeds(callee, i)
	if !ok {
		return values
	}
	o := c.litOrigin(lit)
	return slices.DeleteFunc(slices.Clone(values), func(v flowArg) bool {
		for p, params := range feeds {
			if slices.Contains(params, v.param) && c.paramOut(h, o.argParam(p)) {
				return false
			}
		}
		return true
	})
}

// closureStoredFed are the callee arguments the closure in slot i stores into a capture: the callee passes them to a
// closure parameter that paramOut says its body puts somewhere, so the caller's flow reaches that capture.
func (c *borrowCheck) closureStoredFed(info *sem.FnInfo, callee *midir.Func, i int, lit *midir.Inst, args []midir.LocalID) []flowArg {
	h := c.p.ByEnt[lit.Ent]
	if callee == nil || h == nil || info == nil {
		return nil
	}
	feeds, ok := c.callableFeeds(callee, i)
	if !ok {
		return nil
	}
	o := c.litOrigin(lit)
	var out []flowArg
	for p, params := range feeds {
		if !c.paramOut(h, o.argParam(p)) {
			continue
		}
		for _, j := range params {
			if j >= len(args) || j >= len(c.declaredParams(callee.Ent)) {
				continue
			}
			// A direct record or borrow is already carried by the ordinary call flow; only a
			// call result needs the closure capture as an explicit destination.
			if d := c.definition(c.valueSource(args[j])); d == nil || d.Op != midir.OpCall && d.Op != midir.OpCallValue && d.Op != midir.OpFutureNew {
				continue
			}
			tie := tieValue
			if c.p.R.Types.Kind(c.p.R.Entity(c.declaredParams(callee.Ent)[j]).Type) == sem.KRef {
				tie = tieRef
			}
			out = append(out, flowArg{arg: args[j], tie: tie, param: j})
		}
	}
	return out
}

// callableFeeds lists, for each argument position of the calls g makes through its parameter f, the parameters of g
// passed there; false when f is used otherwise or an argument does not come from a parameter.
func (c *borrowCheck) callableFeeds(g *midir.Func, f int) ([][]int, bool) {
	if f >= len(g.Params) {
		return nil, false
	}
	defs := mirflow.SoleDefs(g)
	var feeds [][]int
	for _, blk := range g.Blocks {
		if slices.ContainsFunc(blk.Term.Args, func(a midir.LocalID) bool { return paramIndex(g, defs, a) == f }) {
			return nil, false
		}
		for _, in := range blk.Insts {
			uses := slices.ContainsFunc(in.Args, func(a midir.LocalID) bool { return paramIndex(g, defs, a) == f })
			switch {
			case !uses || in.Op == midir.OpCopy || in.Op == midir.OpMove || in.Op == midir.OpAlias || in.Op == midir.OpDrop || in.Op == midir.OpRelease:
			case in.Op == midir.OpCallValue && paramIndex(g, defs, in.Args[0]) == f && !slices.ContainsFunc(in.Args[1:], func(a midir.LocalID) bool { return paramIndex(g, defs, a) == f }):
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
