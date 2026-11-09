package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// closureLiteral is the closure literal l is a copy of.
func (c *borrowCheck) closureLiteral(l midir.LocalID) *midir.Inst {
	if d := c.defThrough(l, midir.OpClosure); d != nil && c.p.R.Entity(d.Ent).Kind == sem.EntClosure {
		return d
	}
	return nil
}

// defThrough is the instruction of opcode op that l is a copy of.
func (c *borrowCheck) defThrough(l midir.LocalID, op midir.Opcode) *midir.Inst {
	for range 8 {
		d := c.soleDef(l)
		switch {
		case d == nil:
			return nil
		case d.Op == op:
			return d
		case len(d.Args) == 0 || d.Op != midir.OpCopy && d.Op != midir.OpMove && d.Op != midir.OpAlias:
			return nil
		}
		l = d.Args[0]
	}
	return nil
}

// capturesHolder reports a closure that captures a `&mut`, or a value keeping one, so running it may store there.
func (c *borrowCheck) capturesHolder(l midir.LocalID) bool {
	lit := c.closureLiteral(l)
	return lit != nil && slices.ContainsFunc(lit.Args, func(a midir.LocalID) bool { return c.storesMutRef(c.f.Locals[a].Type) })
}

// captureReach adds, for a closure the callee may run, the places its body may store into through each capture; a
// holder this frame took as a parameter reaches its caller's places, which addCaller checks as such.
func (c *borrowCheck) captureReach(lit *midir.Inst, add, addCaller func(midir.LocalID)) {
	h, o := c.p.ByEnt[lit.Ent], c.litOrigin(lit)
	for j, a := range lit.Args {
		sites := c.paramStoreSites(h, o.boundParam(0, j))
		if len(sites) == 0 {
			continue
		}
		root := c.valueSource(a)
		if c.p.R.Types.Kind(c.f.Locals[a].Type) == sem.KRef {
			c.captureRefReach(lit, j, add, addCaller)
			continue
		}
		if p, ok := c.heldOrigin(root); ok && slices.ContainsFunc(sites, func(s paramStore) bool { return s.via }) {
			addCaller(p)
		}
		for _, u := range c.siteOwners(root, sites) {
			if c.callerPlace(u) || c.keepsBorrows(u) {
				add(u)
			}
		}
	}
}

func (c *borrowCheck) captureRefReach(lit *midir.Inst, j int, add, addCaller func(midir.LocalID)) {
	a := lit.Args[j]
	if p, ok := c.heldRead(a); ok {
		addCaller(p)
		return
	}
	fl := storeFlow{target: a, ref: true, callee: lit.Ent, param: c.litOrigin(lit).boundParam(0, j), args: lit.Args}
	if root, _, caller, ok := c.storeRoot(fl); ok && caller {
		addCaller(root)
		return
	}
	dests, callers := c.storeReach(fl)
	for _, d := range dests {
		add(d)
	}
	for _, p := range callers {
		addCaller(p)
	}
}

// callableMayStore reports whether code passed as l may store through what it is called with: each origin calleeOrigins
// names answers by its body's summaries; an unknown origin, a parameter or a body-less fn may.
func (c *borrowCheck) callableMayStore(l midir.LocalID) bool {
	origins, ok := c.calleeOrigins(c.f, c.soleDefMap(), l)
	return !ok || slices.ContainsFunc(origins, func(o calleeOrigin) bool {
		return len(c.callableSites(o.body, o.argParams(), siteStores)) > 0
	})
}

// fedOrigins are the borrow-carrying locals a closure-fed value was built from, so a store through a capture
// checks the source rather than the container whose flowInfo is not a borrow origin.
func (c *borrowCheck) fedOrigins(l midir.LocalID, depth int) []flowArg {
	l = c.valueSource(l)
	if depth <= 0 {
		return []flowArg{{arg: l}}
	}
	d := c.definition(l)
	if d == nil {
		if c.p.R.Types.Kind(c.f.Locals[l].Type) == sem.KRef || c.f.Locals[l].Borrowed || c.keepsBorrows(l) {
			return []flowArg{{arg: l}}
		}
		return nil
	}
	switch d.Op {
	case midir.OpCopy, midir.OpMove, midir.OpAlias, midir.OpField, midir.OpIndex, midir.OpPayload, midir.OpUnbox:
		if len(d.Args) > 0 {
			return c.fedOrigins(d.Args[0], depth-1)
		}
	case midir.OpCall, midir.OpCallValue, midir.OpFutureNew:
		var out []flowArg
		for _, a := range d.Args {
			out = append(out, c.fedOrigins(a, depth-1)...)
		}
		if out != nil {
			return out
		}
	}
	return []flowArg{{arg: l}}
}

// captureValues are the captures a closure's body may store under another capture, and the parts of a capture it may
// store back under the same one; param -1 keeps the callee's own summaries off them.
func (c *borrowCheck) captureValues(lit *midir.Inst) []flowArg {
	h, o := c.p.ByEnt[lit.Ent], c.litOrigin(lit)
	var out []flowArg
	for i, a := range lit.Args {
		contents := c.p.R.Types.Kind(c.f.Locals[a].Type) == sem.KRef
		for j := range lit.Args {
			if j != i && len(c.paramStoreSites(h, o.boundParam(0, j))) > 0 {
				out = append(out, flowArg{arg: a, contents: contents, param: -1})
				break
			}
		}
		if at := c.paramSelfSources(h, o.boundParam(0, i)); at != nil {
			out = append(out, flowArg{arg: a, contents: contents, param: -1, at: at})
		}
	}
	return out
}
