package mir

import (
	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
)

type refScopeKey struct{ fn, param sem.EntityID }

func (state *borrowAnalysis) checkRefScopes() {
	p := state.Program
	c := &borrowCheck{p: p, state: state}
	memo, active := map[refScopeKey]bool{}, map[refScopeKey]bool{}
	seen := map[[3]uint32]bool{}
	for _, u := range p.R.RefScopeUses() {
		if u.Owner != 0 && p.ByEnt[u.Owner] == nil || u.Owner == 0 && p.Unlowered(u.Target) {
			continue
		}
		key := [3]uint32{uint32(u.File), uint32(u.Node), uint32(u.Target)}
		if seen[key] {
			continue
		}
		keys, name, safe := state.refScopeOrigins(u)
		for _, key := range keys {
			safe = safe && c.refStaysInResult(key, memo, active)
		}
		if !safe {
			seen[key] = true
			code, _ := sem.CodeByName("borrow-callable-lifetime")
			t := p.R.Tree(u.File)
			d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(u.Node)), Code: code.Num,
				Msg: sem.FillTemplate(code.Template, name)}
			p.Diags = append(p.Diags, FileDiag{File: u.File, Diag: d})
		}
	}
}

// Losing a named reference contract requires proof that the reference only leaves through the result.
func (c *borrowCheck) refStaysInResult(key refScopeKey, memo, active map[refScopeKey]bool) bool {
	if ok, found := memo[key]; found {
		return ok
	}
	if active[key] {
		return false
	}
	active[key] = true
	ok := c.refResultOnly(key, memo, active)
	delete(active, key)
	memo[key] = ok
	return ok
}

func (c *borrowCheck) refResultOnly(key refScopeKey, memo, active map[refScopeKey]bool) bool {
	g := c.p.ByEnt[key.fn]
	if g == nil {
		return false
	}
	fs, kept, ok := c.finishedFlow(g)
	if !ok {
		return false
	}
	for _, l := range g.Params {
		if kept[l][uint32(key.param)] {
			return false
		}
	}
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpDrop || in.Op == OpRelease {
				for _, a := range in.Args {
					if fs[a][uint32(key.param)] && (c.typeRuns(g.Locals[a].Type, runsDrop) || c.holdsDeferred(g.Locals[a].Type)) {
						return false
					}
				}
			}
			if in.Op == OpGlobalSet && fs[in.Args[0]][uint32(key.param)] {
				return false
			}
			if bodyRoles[in.Op] != roleCall {
				continue
			}
			ents, params, unseen := c.calleeOf(g, fs, in)
			for k, a := range in.Args {
				if !fs[a][uint32(key.param)] || !c.carries(g.Locals[a].Type) {
					continue
				}
				if unseen || len(params) != 0 || len(ents) == 0 {
					return false
				}
				for _, e := range ents {
					qs := c.boundParams([]sem.EntityID{e}, in, k)
					if len(qs) == 0 {
						return false
					}
					for _, q := range qs {
						if c.p.ByEnt[e] == nil && c.scopedBorrow(q) {
							continue
						}
						if !c.refStaysInResult(refScopeKey{e, q}, memo, active) {
							return false
						}
					}
				}
			}
		}
	}
	return true
}

func (c *borrowCheck) scopedBorrow(q sem.EntityID) bool {
	tt := c.p.R.Types
	e := c.p.R.Entity(q)
	t := e.Type
	if n := tt.Node(t); n.Kind == sem.KRef {
		t = n.Elem
	} else if e.Flags&sem.EfSelf == 0 {
		return false
	}
	return c.p.R.Local(q).Lifetime == 0 && !c.holdsBorrow(t) && !c.holdsDeferred(t) && !c.namesTypeParam(t)
}
