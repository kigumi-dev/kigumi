package sem

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
		if con.Kind == CFn {
			for _, cap := range info.Captures {
				if cap.Mode == capCell {
					if lentReceiver(&c.r.Entities[cap.Local]) {
						return "the closure captures the lent receiver `" + c.r.Entities[cap.Local].Name + "`"
					}
					return "the closure assigns `" + c.r.Entities[cap.Local].Name + "`; it is `FnMut`"
				}
			}
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
	g, w := tt.Node(c.vars.resolve(got)), tt.Node(c.vars.resolve(want))
	for i := range g.Args {
		if tt.Kind(w.Args[i]) == KFn && tt.Kind(g.Args[i]) == KFn && !tt.FnIsEscaping(w.Args[i]) && tt.FnIsEscaping(g.Args[i]) {
			return i, true
		}
	}
	return 0, false
}
