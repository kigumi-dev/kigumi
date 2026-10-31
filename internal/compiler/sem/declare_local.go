package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) declareLocal(n syntax.NodeID, name string, typ TypeID, mut bool) EntityID {
	e := Entity{Kind: EntLocal, Name: name, Pkg: c.pkg, File: c.f, Node: n, Tok: c.t.View(n).Tok().Index(), Parent: c.fn, Type: typ}
	if mut {
		e.Flags |= EfMut
	}
	if c.r.Scopes[c.scope].Kind == ScopeScript {
		e.Flags |= EfScript
	}
	id := c.r.newEntity(e)
	c.r.Entities[id].Detail = c.r.addLocal(LocalInfo{Scope: c.scope})
	c.info.Defs[n] = id
	c.locals = append(c.locals, id)
	c.consumed.declare(id)
	if name == "nil" {
		c.errAt(n, cNilName)
	}
	if name != "_" {
		c.r.Scopes[c.scope].Names[name] = Binding{Ent: id}
	}
	return id
}

// dropsAtScopeExit emits every owning binding's EdgeDrop except one already consumed.
func (c *checker) dropsAtScopeExit(params []EntityID) {
	c.consumed.commit()
	for _, id := range params {
		c.implicitDrop(id)
	}
	for _, id := range c.locals {
		c.implicitDrop(id)
	}
}

// implicitDrop over-approximates: a binding moved out some other way still drops wherever that reaches, so this never loses soundness.
func (c *checker) implicitDrop(id EntityID) {
	e := &c.r.Entities[id]
	if c.consumed.whole[id] {
		return
	}
	// A closure's Copy-ness isn't settled until checkContextRequirements runs, so
	// always record the drop here; dropEffect later finds it trivial once it is.
	if c.r.Types.Kind(e.Type) != KClosure && c.r.isCopy(e.Type) {
		return
	}
	c.edge(EffectEdge{Kind: EdgeDrop, Type: e.Type, Node: e.Node})
}

// consumeWhole marks a bare local/param identifier consumed on an unconditional whole-value use; callers decide what counts as consuming.
func (c *checker) consumeWhole(a syntax.NodeID) {
	c.reserveUse(a)
	m, ok := c.movedOut(a)
	if !ok {
		return
	}
	c.noteExitTake(m)
	if c.branch == 0 && c.loop == 0 && !c.consumed.exitedSince(m, c.consumed.born[m.ent]) {
		c.consumed.pending = append(c.consumed.pending, pendingMove{m, len(c.consumed.exits)})
	}
}

// unconsume un-marks a binding assignment just gave a fresh value to drop.
func (c *checker) unconsume(id EntityID) {
	c.consumed.clear(id)
}

func (c *checker) isResultFn() bool {
	_, _, ok := c.r.Types.IsResult(c.retType)
	return ok
}
