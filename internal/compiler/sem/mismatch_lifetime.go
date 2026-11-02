package sem

import (
	"slices"
	"strings"
)

// mismatchText labels want and got for E400. A lifetime the function being checked does not declare is qualified by
// the item that declares it; when the two read the same, unbound lifetime variables are shown as what they instantiate.
func (c *checker) mismatchText(want, got TypeID) (string, string) {
	wl, gl := c.vars.literalLabel(want), c.vars.literalLabel(got)
	if qualify := c.foreignLifetimes(wl, gl); len(qualify) > 0 {
		ws, gs := c.lifetimeString(wl, qualify), c.lifetimeString(gl, qualify)
		if ws != gs {
			return ws, gs
		}
	} else if ws, gs := c.r.mismatchLabels(wl, gl); ws != gs {
		return ws, gs
	}
	wn, gn := c.vars.namedLifetimes(want), c.vars.namedLifetimes(got)
	qualify := c.foreignLifetimes(wn, gn)
	return c.lifetimeString(wn, qualify), c.lifetimeString(gn, qualify)
}

// typeLabel is t for a diagnostic in the terms of the function being checked.
func (c *checker) typeLabel(t TypeID) string {
	t = c.vars.namedLifetimes(c.vars.defaulted(t))
	return c.lifetimeString(t, c.foreignLifetimes(t))
}

// typeArgLabel keeps a poisoned type as a TypeID, so the diagnostic is still suppressed for it.
func (c *checker) typeArgLabel(t TypeID) any {
	if t == TyPoison || t == NoType {
		return t
	}
	return c.typeLabel(t)
}

// foreignTypeArg labels t only when it names a lifetime from another scope; otherwise t stays a TypeID, which the
// display help inspects.
func (c *checker) foreignTypeArg(t TypeID) any {
	if len(c.foreignLifetimes(c.vars.namedLifetimes(c.vars.defaulted(t)))) == 0 {
		return t
	}
	return c.typeLabel(t)
}

func (c *checker) foreignLifetimes(ts ...TypeID) map[EntityID]bool {
	var own []EntityID
	if c.fn != 0 && c.r.Entities[c.fn].Kind == EntFn {
		own = c.r.Fn(c.fn).TypeParams
	}
	qualify := map[EntityID]bool{}
	for _, t := range ts {
		for _, p := range c.r.Types.Params(t) {
			if c.r.typeParam(p).IsLifetime && !slices.Contains(own, p) {
				qualify[p] = true
			}
		}
	}
	return qualify
}

func (c *checker) lifetimeString(t TypeID, qualify map[EntityID]bool) string {
	var sb strings.Builder
	c.r.writeTypeQualifyingLifetimes(&sb, t, qualify)
	return sb.String()
}

// namedLifetimes resolves t with each unbound lifetime variable standing for the parameter it instantiates.
func (v *varStore) namedLifetimes(t TypeID) TypeID {
	t = v.resolve(t)
	tt := v.r.Types
	if t == 0 || !tt.ContainsVar(t) {
		return t
	}
	n := tt.Node(t)
	if n.Kind == KVar {
		if p, ok := v.lifetimeParams[n.Var]; ok {
			return tt.Param(p)
		}
		return t
	}
	n.Args = slices.Clone(n.Args)
	for i, a := range n.Args {
		n.Args[i] = v.namedLifetimes(a)
	}
	n.Elem, n.Lifetime = v.namedLifetimes(n.Elem), v.namedLifetimes(n.Lifetime)
	return tt.Intern(n)
}
