package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// calleeOrigin is one thing a call through a function value runs: a parameter of the caller (held, a callable read out
// of it), or the fn or closure ent with the operands its value binds around the call's arguments (layers, innermost first).
type calleeOrigin struct {
	param   int
	isParam bool
	held    bool
	ent     sem.EntityID
	body    *midir.Func
	layers  []binding
	known   bool
}

func (c *borrowCheck) originOf(e sem.EntityID, layers ...binding) calleeOrigin {
	return calleeOrigin{ent: e, body: c.p.ByEnt[e], layers: layers, known: true}
}

// calleeOrigin is the parameter, closure, method value, bind or applied method value l copies or is read out of a
// parameter; fn items are left to fnItemOrigins, and a witness method value runs a body only its witness knows.
func (c *borrowCheck) calleeOrigin(g *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID) calleeOrigin {
	if i := paramIndex(g, defs, l); i >= 0 {
		if c.assignedParam(g, g.Params[i]) {
			return calleeOrigin{}
		}
		return calleeOrigin{param: i, isParam: true}
	}
	if i, ok := c.heldParam(g, defs, l); ok {
		return calleeOrigin{param: i, isParam: true, held: true}
	}
	l = copiedFrom(g, defs, l)
	d := defs[l]
	if d == nil || g.Locals[l].Cell {
		return calleeOrigin{}
	}
	if d.Op == midir.OpBind {
		return c.originOf(d.Ent, bindingOf(d, d.Args))
	}
	switch d.Form() {
	case midir.FormApplied:
		if len(d.Args) != 2 {
			return calleeOrigin{}
		}
		w := defs[copiedFrom(g, defs, d.Args[0])]
		if w == nil || w.Op != midir.OpBind || w.Ent != d.Ent {
			return calleeOrigin{}
		}
		return c.originOf(d.Ent, bindingOf(d, d.Args[1:]), bindingOf(w, w.Args))
	case midir.FormClosure, midir.FormMethod:
		return c.originOf(d.Ent, bindingOf(d, d.Args))
	}
	return calleeOrigin{}
}

// calleeOrigins are what the callable local l of g may run, the one answer every check of a value call reads: a
// parameter, each fn item fnItemOrigins names, or calleeOrigin's value; false when they are unknown.
func (c *borrowCheck) calleeOrigins(g *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID) ([]calleeOrigin, bool) {
	if o := c.calleeOrigin(g, defs, l); o.known || o.isParam {
		return []calleeOrigin{o}, true
	}
	items := c.within(g).fnItemOrigins(l, 8)
	if !items.known || len(items.ents) == 0 {
		return nil, false
	}
	var out []calleeOrigin
	for i, e := range items.ents {
		if !slices.Contains(items.ents[:i], e) {
			out = append(out, c.originOf(e))
		}
	}
	return out, true
}

// soleOrigin is the one fn or closure the callable local l of this body runs; false for a parameter, several or unknown.
func (c *borrowCheck) soleOrigin(l midir.LocalID) (calleeOrigin, bool) {
	origins, ok := c.calleeOrigins(c.f, c.soleDefMap(), l)
	if !ok || len(origins) != 1 || origins[0].isParam {
		return calleeOrigin{}, false
	}
	return origins[0], true
}

func (c *borrowCheck) litOrigin(lit *midir.Inst) calleeOrigin {
	return c.originOf(lit.Ent, bindingOf(lit, lit.Args))
}

func (c *borrowCheck) within(g *midir.Func) *borrowCheck {
	if g == c.f {
		return c
	}
	return &borrowCheck{p: c.p, f: g, state: c.state}
}

func (c *borrowCheck) frameCallee(g *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID, held bool, depth int) bool {
	if !held && !c.calleeKeepsBorrow(g.Locals[l].Type) {
		return false
	}
	origins, ok := c.argOrigins(g, defs, l, held)
	if !ok || depth == 0 {
		return true
	}
	for _, o := range origins {
		if o.isParam || o.body == nil {
			return true
		}
		for _, s := range c.callableSites(o.body, o.argParams(), siteOut) {
			if c.frameTarget(g, defs, s.callee, o.boundLocals(), depth-1) {
				return true
			}
		}
	}
	return false
}
