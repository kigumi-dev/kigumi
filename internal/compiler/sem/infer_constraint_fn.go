package sem

import "slices"

func (c *checker) fnUnsatisfied(arg TypeID, con Constraint) string {
	tt := c.r.Types
	switch tt.Kind(arg) {
	case KFn:
		if !c.sameFnShape(arg, con.Type) {
			return "signature differs"
		}
		return c.fnEscapeMismatch(arg, con.Type)
	case KClosure:
		info := c.r.closure(tt.Node(arg).Ent)
		if !c.sameFnShape(info.Sig, con.Type) {
			return "signature differs"
		}
		if why := c.fnEscapeMismatch(info.Sig, con.Type); why != "" {
			return why
		}
		if con.Kind != CFnOnce && info.Once != "" {
			return "the closure consumes `" + info.Once + "`; it is `FnOnce`"
		}
		if con.Kind == CFn {
			return c.fnMutWhy(tt.Node(arg).Ent, map[EntityID]bool{})
		}
		return ""
	case KParam:
		var why string
		for _, own := range c.r.typeParam(tt.Node(arg).Ent).Constraints {
			if own.Kind >= CFn && own.Kind <= con.Kind && c.sameFnShape(own.Type, con.Type) {
				if m := c.fnEscapeMismatch(own.Type, con.Type); m != "" {
					why = m
					continue
				}
				return ""
			}
		}
		if why != "" {
			return why
		}
		return "add the constraint to `" + c.r.TypeString(arg) + "`"
	}
	return "it is not callable"
}

// fnEscapeMismatch is escapeMismatch (conform.go) for Fn/FnMut/FnOnce constraints, where got/want
// are whole KFn types; run after sameFnShape already confirmed the shapes match ignoring escaping.
func (c *checker) fnEscapeMismatch(got, want TypeID) string {
	if i, ok := c.firstEscapeMismatch(got, want); ok {
		return c.r.noteText("may keep the callback at parameter {n}, which the constraint requires not to escape", i+1)
	}
	return ""
}

// firstEscapeMismatch is the shared one-directional check behind fnEscapeMismatch and coerceToFn:
// got may be more escaping than want at a parameter, never less. Reports the first mismatch index.
func (c *checker) firstEscapeMismatch(got, want TypeID) (int, bool) {
	tt := c.r.Types
	got, want = c.vars.resolve(got), c.vars.resolve(want)
	g := tt.Node(got)
	for i := range g.Args {
		if c.paramEscapes(got, i) && !c.paramEscapes(want, i) {
			return i, true
		}
	}
	return 0, false
}

func (c *checker) paramEscapes(fn TypeID, i int) bool {
	tt := c.r.Types
	n := tt.Node(fn)
	if n.Kind != KFn || i >= len(n.Args) {
		return false
	}
	return tt.FnParamEscapes(fn, i) || tt.FnIsEscaping(c.vars.resolve(n.Args[i]))
}

// fnMutWhy says why closure is at most `FnMut`, "" when it is `Fn`: it captures a cell (written inside it or
// outside it), or a closure or callable parameter that is itself `FnMut`, which running it runs.
func (c *checker) fnMutWhy(closure EntityID, seen map[EntityID]bool) string {
	if seen[closure] {
		return ""
	}
	seen[closure] = true
	for _, cap := range c.r.closure(closure).Captures {
		name := c.r.Entities[cap.Local].Name
		switch {
		case cap.Mode == capCell && lentReceiver(&c.r.Entities[cap.Local]):
			return "the closure captures the lent receiver `" + name + "`"
		case cap.Mode == capCell && c.closureWrites(closure, cap.Local):
			return "the closure assigns `" + name + "`; it is `FnMut`"
		case cap.Mode == capCell:
			return "the closure shares `" + name + "` through a cell, since it is written outside the closure; it is `FnMut`"
		case c.callsFnMut(c.r.Entities[cap.Local].Type, seen):
			return "the closure captures `" + name + "`, which is `FnMut`; it is `FnMut` too"
		}
	}
	return ""
}

// callsFnMut reports a captured callable that is at most `FnMut`: a closure that is, or a parameter bound `FnMut`;
// one bound only `FnOnce` cannot be called from a closure at all, which is reported where it is called.
func (c *checker) callsFnMut(t TypeID, seen map[EntityID]bool) bool {
	n := c.r.Types.Node(c.vars.resolve(t))
	switch n.Kind {
	case KClosure:
		return n.Ent != 0 && c.r.Entities[n.Ent].Kind == EntClosure && c.fnMutWhy(n.Ent, seen) != ""
	case KParam:
		mut := false
		for _, con := range c.r.typeParam(n.Ent).Constraints {
			if con.Kind == CFn || con.Kind == CFnOnce {
				return false
			}
			mut = mut || con.Kind == CFnMut
		}
		return mut
	}
	return false
}

func (c *checker) cellCaptureWhy(closure, ent EntityID) string {
	switch {
	case lentReceiver(&c.r.Entities[ent]):
		return "a receiver lent by the caller"
	case c.closureWrites(closure, ent):
		return "assigned inside the closure"
	}
	return "written outside this closure, so the closure shares it through a cell"
}

func (c *checker) closureWrites(closure, ent EntityID) bool {
	return slices.Contains(c.held.closureWrites[closure], ent)
}
