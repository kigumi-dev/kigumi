package sem

import "kigumi/internal/compiler/syntax"

// declareLocal declares a source-level local binding in the current scope. A
// name already bound by a local or parameter in that same scope is a
// redeclaration error (E258); an inner scope may still shadow an outer one.
func (c *checker) declareLocal(n syntax.NodeID, name string, typ TypeID, mut bool) EntityID {
	return c.bindLocal(n, name, typ, mut, true)
}

// declareLocalDup declares a binding whose repeat inside one pattern the
// pattern checker already reported (E809); rechecking here would duplicate it.
func (c *checker) declareLocalDup(n syntax.NodeID, name string, typ TypeID, mut bool) EntityID {
	return c.bindLocal(n, name, typ, mut, false)
}

// declareInternalLocal declares a compiler-synthesized binding whose name
// cannot be written in source (`$ctxN`, `?`). Such names are never resolved by
// name and may repeat in one scope, so they skip the redeclaration check.
func (c *checker) declareInternalLocal(n syntax.NodeID, name string, typ TypeID, mut bool) EntityID {
	return c.bindLocal(n, name, typ, mut, false)
}

func (c *checker) bindLocal(n syntax.NodeID, name string, typ TypeID, mut bool, redeclare bool) EntityID {
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
		if redeclare {
			c.redeclaredLocal(n, name)
		}
		c.r.Scopes[c.scope].Names[name] = Binding{Ent: id}
	}
	return id
}

// redeclaredLocal reports E258 when name is already bound by a local or
// parameter declared in the current scope. Outer scopes are shadowable, except
// that a callable's parameters and its outermost body block are one lexical
// declaration scope: `fn f(x: Int) { let x = ... }` redeclares `x`. A block
// nested inside that body is a new scope and may shadow the parameter.
func (c *checker) redeclaredLocal(n syntax.NodeID, name string) bool {
	if c.patOr > 0 {
		return false
	}
	if c.localIn(c.scope, name) {
		c.errAt(n, cLocalRedeclared, name)
		return true
	}
	s := &c.r.Scopes[c.scope]
	if s.Kind != ScopeBlock {
		return false
	}
	if c.r.Scopes[s.Parent].Params && c.localIn(s.Parent, name) {
		c.errAt(n, cLocalRedeclared, name)
		return true
	}
	return false
}

// localIn reports whether scope itself binds name to a local or parameter.
func (c *checker) localIn(scope ScopeID, name string) bool {
	b, ok := c.r.Scopes[scope].Names[name]
	if !ok || b.Ent == 0 {
		return false
	}
	switch c.r.Entities[b.Ent].Kind {
	case EntLocal, EntParam:
	default:
		return false
	}
	return c.r.local(b.Ent).Scope == scope
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
