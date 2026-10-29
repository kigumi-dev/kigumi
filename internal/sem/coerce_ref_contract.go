package sem

import "kigumi/internal/syntax"

type RefScopeUse struct {
	Owner, Target EntityID
	Params        []EntityID
	Slots         []int
	File          FileID
	Node          syntax.NodeID
}

var cRefScopeErase = defineCode("E2297", "borrow-callable-lifetime", "the reference lifetime of `{name}` cannot be erased: the callable may retain that reference outside its result")

func (tt *TypeTable) erasedRefParams(got, want TypeID) []int {
	g, w := tt.Node(got), tt.Node(want)
	if g.Kind != KFn || w.Kind != KFn || len(g.Args) != len(w.Args) {
		return nil
	}
	var out []int
	for i, p := range g.Args {
		if tt.RefLifetime(p) != 0 && tt.Kind(w.Args[i]) == KRef && tt.RefLifetime(w.Args[i]) == 0 {
			out = append(out, i)
		}
	}
	return out
}

func (r *Result) refScopeUse(owner, target EntityID, got, want TypeID, file FileID, node syntax.NodeID) RefScopeUse {
	u := RefScopeUse{Owner: owner, Target: target, File: file, Node: node}
	var ps []EntityID
	if r.Entities[target].Kind == EntClosure {
		ps = r.closure(target).Params
	} else {
		ps = r.Fn(target).Params
	}
	for _, i := range r.Types.erasedRefParams(got, want) {
		if i < len(ps) {
			u.Params = append(u.Params, ps[i])
		}
	}
	return u
}

func (c *checker) refScopeContract(n syntax.NodeID, got, want TypeID) {
	tt := c.r.Types
	got, want = c.vars.resolve(got), c.vars.resolve(want)
	var targets []EntityID
	if item := c.info.Uses[c.t.Unparen(n)]; item != 0 && c.r.Entities[item].Kind == EntFn {
		targets, got = []EntityID{item}, c.r.Fn(item).Sig
	} else if tt.Kind(got) == KClosure {
		ent := tt.Node(got).Ent
		targets, got = []EntityID{ent}, c.r.closure(ent).Sig
	} else {
		targets = c.localKnownFns(n)
		if tt.Kind(got) == KParam {
			got = c.r.fnShapeOf(got)
		}
	}
	if len(tt.erasedRefParams(got, want)) == 0 {
		return
	}
	owner := c.fn
	if len(c.lambdas) > 0 {
		owner = c.lambdas[len(c.lambdas)-1].ent
	}
	if len(targets) == 0 {
		c.r.refUses = append(c.r.refUses, RefScopeUse{Owner: owner, Slots: tt.erasedRefParams(got, want), File: c.f, Node: c.t.Unparen(n)})
	}
	for _, target := range targets {
		c.r.refUses = append(c.r.refUses, c.r.refScopeUse(owner, target, got, want, c.f, n))
	}
}

func (r *Result) RefScopeUses() []RefScopeUse {
	out := append([]RefScopeUse(nil), r.refUses...)
	for _, p := range r.witnessPairs {
		u := r.refScopeUse(0, p.m, r.Fn(p.m).Sig, r.Fn(p.req).Sig, r.Entities[p.m].File, r.Entities[p.m].Node)
		if len(u.Params) > 0 {
			out = append(out, u)
		}
	}
	return out
}
