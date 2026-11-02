package borrow

import (
	"fmt"
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
)

type refScopeKey struct{ fn, param sem.EntityID }

func (state *Analyzer) CheckRefScopes() []midir.FileDiag {
	var out []midir.FileDiag
	p := state.Input
	c := &borrowCheck{p: p, state: state}
	memo, active := map[refScopeKey]bool{}, map[refScopeKey]bool{}
	seen, failed := map[[3]uint32]bool{}, map[string][][2]int{}
	emitted := map[[2]uint32][][2]int{}
	for _, u := range p.R.RefScopeUses() {
		if u.Owner != 0 && p.ByEnt[u.Owner] == nil || u.Owner == 0 && p.Unlowered(u.Target) {
			continue
		}
		key := [3]uint32{uint32(u.File), uint32(u.Node), uint32(u.Target)}
		if seen[key] {
			continue
		}
		keys, name, safe := state.refScopeOrigins(u)
		var failing []refScopeKey
		for _, key := range keys {
			if !c.refStaysInResult(key, memo, active) {
				safe, failing = false, append(failing, key)
			}
		}
		t := p.R.Tree(u.File)
		// A value and the literal holding it that fail on the same callables are one error, whichever bound asked.
		s := t.Span(u.Node)
		group, span := fmt.Sprint(u.Owner, u.File, failing), [2]int{int(s.Start), int(s.End)}
		ekey := [2]uint32{uint32(u.Owner), uint32(u.File)}
		// An unknown-origin use adds nothing where a known callable already failed at the same node.
		covered := len(failing) == 0 && slices.ContainsFunc(emitted[ekey], func(o [2]int) bool { return nested(span, o) })
		if !safe && !covered && !slices.ContainsFunc(failed[group], func(o [2]int) bool { return nested(o, span) || nested(span, o) }) {
			seen[key], failed[group] = true, append(failed[group], span)
			emitted[ekey] = append(emitted[ekey], span)
			code, _ := sem.CodeByName("borrow-callable-lifetime")
			d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(u.Node)), Code: code.Num,
				Msg: sem.FillTemplate(code.Template, name)}
			out = append(out, midir.FileDiag{File: u.File, Diag: d})
		}
	}
	return out
}

func nested(in, out [2]int) bool { return out[0] <= in[0] && in[1] <= out[1] }

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
		if kept[l][uint32(key.param)] && !c.ownPlaceHoldsNothing(g, l, key.param) {
			return false
		}
	}
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpDrop || in.Op == midir.OpRelease {
				for _, a := range in.Args {
					if fs[a][uint32(key.param)] && (c.typeRuns(g.Locals[a].Type, runsDrop) || c.holdsDeferred(g.Locals[a].Type)) {
						return false
					}
				}
			}
			if in.Op == midir.OpGlobalSet && fs[in.Args[0]][uint32(key.param)] {
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
						if c.p.ByEnt[e] == nil && (c.scopedBorrow(q) || c.bodylessKeepsNone(g, in, e)) {
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
	return c.p.R.Local(q).Lifetime == 0 && !c.facts().HoldsBorrow(t) && !c.holdsDeferred(t) && !c.facts().NamesTypeParam(t)
}
