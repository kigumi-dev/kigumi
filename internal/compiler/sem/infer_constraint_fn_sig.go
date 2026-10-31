package sem

// bindCallableConstraints fits a callable argument's signature to its Fn / FnMut / FnOnce bound
// before unresolved variables are reported, so a stored closure still determines the bound's types.
func (c *checker) bindCallableConstraints() {
	tt := c.r.Types
	for _, cc := range c.constraints {
		arg := c.vars.resolve(cc.arg)
		if tt.ContainsVar(arg) {
			continue
		}
		sig := arg
		switch tt.Kind(arg) {
		case KClosure:
			sig = c.r.closure(tt.Node(arg).Ent).Sig
		case KFn:
		default:
			continue
		}
		for _, con := range c.r.typeParam(cc.param).Constraints {
			if con.Kind == CFn || con.Kind == CFnMut || con.Kind == CFnOnce {
				c.sameFnShape(sig, tt.Subst(con.Type, cc.subst))
			}
		}
	}
}
