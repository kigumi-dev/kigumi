package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// closureLiteral is the closure literal l is a copy of.
func (c *borrowCheck) closureLiteral(l LocalID) *Inst {
	if d := c.defThrough(l, OpClosure); d != nil && c.p.R.Entity(d.Ent).Kind == sem.EntClosure {
		return d
	}
	return nil
}

// defThrough is the instruction of opcode op that l is a copy of.
func (c *borrowCheck) defThrough(l LocalID, op Opcode) *Inst {
	for range 8 {
		d := soleDef(c.f, l)
		switch {
		case d == nil:
			return nil
		case d.Op == op:
			return d
		case len(d.Args) == 0 || d.Op != OpCopy && d.Op != OpMove && d.Op != OpAlias:
			return nil
		}
		l = d.Args[0]
	}
	return nil
}

// capturesHolder reports a closure that captures a `&mut`, or a value keeping one, so running it may store there.
func (c *borrowCheck) capturesHolder(l LocalID) bool {
	lit := c.closureLiteral(l)
	return lit != nil && slices.ContainsFunc(lit.Args, func(a LocalID) bool { return c.storesMutRef(c.f.Locals[a].Type) })
}

// captureReach adds, for a closure the callee may run, the places its body may store into through each capture; a
// holder this frame took as a parameter reaches its caller's places, which addCaller checks as such.
func (c *borrowCheck) captureReach(lit *Inst, add, addCaller func(LocalID)) {
	h := c.p.ByEnt[lit.Ent]
	for j, a := range lit.Args {
		sites := c.paramStoreSites(h, j)
		if len(sites) == 0 {
			continue
		}
		root := c.valueSource(a)
		if c.p.R.Types.Kind(c.f.Locals[a].Type) == sem.KRef {
			root = c.flowPlace(storeFlow{target: a, ref: true})
			add(root)
		} else if slices.Contains(c.f.Params, root) && slices.ContainsFunc(sites, func(s paramStore) bool { return s.via }) {
			addCaller(root)
		}
		for _, u := range c.siteOwners(root, sites) {
			if c.callerPlace(u) || c.keepsBorrows(u) {
				add(u)
			}
		}
	}
}

// callableMayStore reports whether code passed as l may store through what it is called with: a closure literal, fn
// item or bound method (an interface witness) answers by its body's summaries, anything else may.
func (c *borrowCheck) callableMayStore(l LocalID) bool {
	var h *Func
	skip := 0
	if lit := c.closureLiteral(l); lit != nil {
		h, skip = c.p.ByEnt[lit.Ent], len(lit.Args)
	} else if d := c.defThrough(l, OpFnItem); d != nil {
		h = c.p.ByEnt[d.Ent]
	} else if d := c.defThrough(l, OpBind); d != nil {
		h, skip = c.p.ByEnt[d.Ent], len(d.Args)
	}
	if h == nil {
		return true
	}
	for j := skip; j < len(h.Params); j++ {
		if c.paramStores(h, j) || len(c.paramSelfSources(h, j)) > 0 {
			return true
		}
	}
	return false
}

// captureValues are the captures a closure's body may store under another capture, and the parts of a capture it may
// store back under the same one; param -1 keeps the callee's own summaries off them.
func (c *borrowCheck) captureValues(lit *Inst) []flowArg {
	h := c.p.ByEnt[lit.Ent]
	var out []flowArg
	for i, a := range lit.Args {
		contents := c.p.R.Types.Kind(c.f.Locals[a].Type) == sem.KRef
		for j := range lit.Args {
			if j != i && len(c.paramStoreSites(h, j)) > 0 {
				out = append(out, flowArg{arg: a, contents: contents, param: -1})
				break
			}
		}
		if at := c.paramSelfSources(h, i); at != nil {
			out = append(out, flowArg{arg: a, contents: contents, param: -1, at: at})
		}
	}
	return out
}
