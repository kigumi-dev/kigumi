package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// checkThreadBounds re-checks the Send and Sync bounds a callee declares on a parameter's type at every call of this
// body, direct or through a value: a value that reaches such a callee unqualified would cross threads unchecked.
func (c *borrowCheck) checkThreadBounds() {
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			switch {
			case in.Op == midir.OpCall && in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn:
				c.threadBoundArgs(in, in.Ent, in.Args)
			case in.Op == midir.OpCallValue && in.Ent == 0 && len(in.Args) > 0:
				origins, ok := c.calleeOrigins(c.f, c.soleDefMap(), in.Args[0])
				for _, o := range origins {
					if ok && !o.isParam && o.ent != 0 && c.p.R.Entity(o.ent).Kind == sem.EntFn {
						c.threadBoundArgs(in, o.ent, o.params(in.Args[1:]))
					}
				}
			}
		}
	}
}

// threadBoundArgs lines args up with fn's declared parameters, the receiver first, as the call passes them, and checks
// what each Send- or Sync-bound type parameter is instantiated with there.
func (c *borrowCheck) threadBoundArgs(in midir.Inst, fn sem.EntityID, args []midir.LocalID) {
	r := c.p.R
	info := r.Fn(fn)
	var params []sem.TypeID
	if info.Recv != sem.RecvNone {
		params = append(params, r.Entity(info.SelfParam).Type)
	}
	params = append(params, r.Types.Node(info.Sig).Args...)
	for k, want := range params {
		if k >= len(args) {
			continue
		}
		got := c.threadArgType(args[k])
		if got == 0 {
			continue
		}
		var items []sem.EntityID
		if r.Types.Kind(want) == sem.KParam {
			items = c.argFnItems(args[k])
		}
		for _, at := range paramInstances(r.Types, want, got, nil) {
			c.threadBoundAt(in, args[k], at, items)
		}
	}
}

// boundInstance is the type a type parameter of a declared parameter type stands for in an argument's type.
type boundInstance struct {
	param sem.EntityID
	t     sem.TypeID
}

// paramInstances walks decl and got together; a shape they do not share leaves that part unmatched.
func paramInstances(tt *sem.TypeTable, decl, got sem.TypeID, out []boundInstance) []boundInstance {
	if decl == 0 || got == 0 {
		return out
	}
	d, g := tt.Node(decl), tt.Node(got)
	if d.Kind == sem.KParam {
		return append(out, boundInstance{d.Ent, got})
	}
	if d.Kind != g.Kind || d.Ent != g.Ent || len(d.Args) != len(g.Args) {
		return out
	}
	out = paramInstances(tt, d.Elem, g.Elem, out)
	for i := range d.Args {
		out = paramInstances(tt, d.Args[i], g.Args[i], out)
	}
	return out
}

func (c *borrowCheck) threadBoundAt(in midir.Inst, arg midir.LocalID, at boundInstance, items []sem.EntityID) {
	r := c.p.R
	for _, con := range r.TypeParam(at.param).Constraints {
		if con.Kind != sem.CSend && con.Kind != sem.CSync {
			continue
		}
		name := map[bool]string{false: "Send", true: "Sync"}[con.Kind == sem.CSync]
		if why := r.ThreadBoundWhy(at.t, con.Kind == sem.CSync, items, c.f.Ent); why != "" {
			c.reportCatalog(in.Node, arg, "constraint-unsatisfied", r.TypeString(at.t), name, why)
		}
	}
}

// threadArgType is the value type arg passes, past the borrow a lent argument travels in; 0 when it is still open.
func (c *borrowCheck) threadArgType(arg midir.LocalID) sem.TypeID {
	tt := c.p.R.Types
	t := c.f.Locals[arg].Type
	for t != 0 && tt.Kind(t) == sem.KRef {
		t = tt.Node(t).Elem
	}
	switch tt.Kind(t) {
	case sem.KVar, sem.KUntyped, sem.KPoison, sem.KConst:
		return 0
	}
	if t == sem.TyNever {
		return 0
	}
	return t
}

// argFnItems are the function items arg holds, when it holds nothing else.
func (c *borrowCheck) argFnItems(arg midir.LocalID) []sem.EntityID {
	if c.p.R.Types.Kind(c.f.Locals[arg].Type) != sem.KFn {
		return nil
	}
	items := c.fnItemOrigins(arg, 8)
	if !items.known {
		return nil
	}
	for _, e := range items.ents {
		if c.p.R.Entity(e).Kind != sem.EntFn {
			return nil
		}
	}
	return items.ents
}
