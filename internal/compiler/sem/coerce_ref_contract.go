package sem

import (
	"slices"
	"strings"

	"kigumi/internal/compiler/syntax"
)

// RefScopeUse is one erasure to prove. Parts then Results lead from the value to the callables it concerns; Params are
// used only when both are empty. Use, when unprovable, names what this use does that no proof follows.
type RefScopeUse struct {
	Owner, Target EntityID
	Params        []EntityID
	Slots         []int
	Parts         []PartStep
	Results       int
	Unprovable    bool
	Use           string
	File          FileID
	Node          syntax.NodeID
}

var cRefScopeErase = defineCode("E2297", "borrow-callable-lifetime", "the reference lifetime of `{name}` cannot be erased: the callable may retain that reference outside its result")

// Message is the E2297 text for u: a refusal at a use names what that use does in place of what the callable may keep.
func (u RefScopeUse) Message(name string) string {
	if u.Use == "" {
		return FillTemplate(cRefScopeErase.Template, name)
	}
	head, _, _ := strings.Cut(cRefScopeErase.Template, ": ")
	return FillTemplate(head, name) + ": " + u.Use
}

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
	return r.targetUse(RefScopeUse{Owner: owner, Target: target, File: file, Node: node}, r.Types.erasedRefParams(got, want))
}

func (r *Result) targetUse(u RefScopeUse, slots []int) RefScopeUse {
	if u.Target == 0 || len(u.Parts) != 0 || u.Results != 0 {
		return RefScopeUse{Owner: u.Owner, Target: u.Target, Slots: slots, Parts: u.Parts, Results: u.Results, File: u.File, Node: u.Node}
	}
	var ps []EntityID
	if r.Entities[u.Target].Kind == EntClosure {
		ps = r.closure(u.Target).Params
	} else {
		ps = r.Fn(u.Target).Params
	}
	for _, i := range slots {
		if i < len(ps) {
			u.Params = append(u.Params, ps[i])
		}
	}
	return u
}

func (c *checker) refScopeContract(n syntax.NodeID, got, want TypeID) {
	got, want = c.vars.resolve(got), c.vars.resolve(want)
	targets, value := c.erasedValue(n, got, want)
	if value != 0 {
		c.recordErasures(n, targets, c.erasedSlots(value, want))
	}
}

// erasedValue is the callables the value at n can be and the fn type its erasure to want is measured on; the type is 0
// when the value is a type parameter one of whose bounds fits want without an erasure.
func (c *checker) erasedValue(n syntax.NodeID, got, want TypeID) ([]EntityID, TypeID) {
	targets := c.valueTargets(n, got)
	if k := c.r.Types.Kind(got); k == KClosure || k == KParam {
		return targets, c.valueShape(got, want)
	}
	return targets, got
}

func (c *checker) valueTargets(n syntax.NodeID, got TypeID) []EntityID {
	if item := c.info.Uses[c.t.Unparen(n)]; item != 0 && c.r.Entities[item].Kind == EntFn {
		return []EntityID{item}
	}
	if c.r.Types.Kind(got) == KClosure {
		return []EntityID{c.r.Types.Node(got).Ent}
	}
	return c.localKnownFns(n)
}

func (c *checker) recordErasures(n syntax.NodeID, targets []EntityID, slots []eraseSlot) {
	var groups []eraseSlot
	var members [][]int
	for _, s := range slots {
		i := slices.IndexFunc(groups, func(g eraseSlot) bool { return g.results == s.results && slices.Equal(g.path, s.path) })
		if i < 0 {
			i, groups, members = len(groups), append(groups, s), append(members, nil)
		}
		members[i] = append(members[i], s.slot)
	}
	for i, g := range groups {
		u := RefScopeUse{Owner: c.refOwner(), Parts: g.path, Results: g.results, File: c.f, Node: c.t.Unparen(n)}
		if len(targets) == 0 || len(g.path) != 0 {
			c.r.refUses = append(c.r.refUses, c.r.targetUse(u, members[i]))
			continue
		}
		for _, target := range targets {
			u.Target = target
			c.r.refUses = append(c.r.refUses, c.r.targetUse(u, members[i]))
		}
	}
}

// refuseErasure records an erasure at n that no proof can reach.
func (c *checker) refuseErasure(n syntax.NodeID, target EntityID) {
	c.refuseErasureUse(n, target, "")
}

func (c *checker) refuseErasureUse(n syntax.NodeID, target EntityID, use string) {
	c.r.refUses = append(c.r.refUses, RefScopeUse{Owner: c.refOwner(), Target: target, Unprovable: true, Use: use, File: c.f, Node: c.t.Unparen(n)})
}

func (c *checker) refOwner() EntityID {
	if c.walkOwner != 0 {
		return c.walkOwner
	}
	if len(c.lambdas) > 0 {
		return c.lambdas[len(c.lambdas)-1].ent
	}
	return c.fn
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
