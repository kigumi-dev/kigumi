package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) dropSources(g *midir.Func, in midir.Inst) []ownquery.Path {
	if len(in.Args) == 0 || in.Emptied {
		return nil
	}
	a := in.Args[0]
	t := g.Locals[a].Type
	if !c.typeRuns(t, runsOwnedDrop) || !c.mayHoldPlace(g, a) {
		return nil
	}
	if h := c.p.ByEnt[c.directDestructor(t)]; h != nil {
		if in.DropKind == midir.DropPartial {
			return nil
		}
		return c.paramSelfSources(h, 0)
	}
	return []ownquery.Path{{}}
}

func (c *borrowCheck) directDestructor(t sem.TypeID) sem.EntityID {
	n := c.p.R.Types.Node(t)
	if n.Kind != sem.KNamed || c.p.R.Entity(n.Ent).Kind != sem.EntType {
		return 0
	}
	d := c.p.R.TypeDecl(n.Ent)
	if d == nil || d.Form != sem.FormResource || len(d.Variants) > 0 {
		return 0
	}
	for _, f := range c.p.R.AggFields(t) {
		if c.typeRuns(f, runsOwnedDrop) {
			return 0
		}
	}
	return d.Drop
}

// dropFlows: a type parameter's value holds what this frame stored into it, reported at that store, or what a call
// returned, which the callee's bound keeps from its arguments; only its callers' summaries need its drop.
func (c *borrowCheck) dropFlows(in midir.Inst) []storeFlow {
	if len(in.Args) == 0 || c.p.R.Types.Kind(c.f.Locals[in.Args[0]].Type) == sem.KParam {
		return nil
	}
	a := in.Args[0]
	at := c.dropSources(c.f, in)
	if at == nil {
		return nil
	}
	fl := storeFlow{target: a, byValue: true, drop: true, node: in.Node, values: []flowArg{{arg: a, at: at}}}
	if d := c.directDestructor(c.f.Locals[a].Type); c.p.ByEnt[d] != nil {
		fl.callee = d
	} else {
		fl.opaque = true
	}
	return []storeFlow{fl}
}

func (c *borrowCheck) droppedParts(b *borrowInfo, holder, root midir.LocalID) (*borrowInfo, bool) {
	f := c.frameBorrows(b)
	if f == nil {
		return nil, false
	}
	out := *f
	holder = c.valueSource(holder)
	out.owners = slices.DeleteFunc(slices.Clone(f.owners), func(o ownerRef) bool {
		return o.owner == root || !c.resetReaches(holder, o.owner, c.curIdx)
	})
	return &out, len(out.owners) > 0
}
