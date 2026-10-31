package sem

import "kigumi/internal/compiler/syntax"

type localLifetimeKey struct {
	fn   EntityID
	name string
}

// In a function declaring no lifetime an undeclared lifetime name is the body's own:
// nothing typed with it can reach the signature.
func (c *checker) resolveLocalType(n syntax.NodeID) TypeID {
	saved := c.r.localLifetimeFn
	if c.declaresNoLifetime() {
		c.r.localLifetimeFn = c.fn
	}
	t := c.r.resolveType(c.f, c.scope, n, posLocal)
	c.r.localLifetimeFn = saved
	return t
}

func (c *checker) declaresNoLifetime() bool {
	if c.fn == 0 || c.r.Entities[c.fn].Kind != EntFn {
		return false
	}
	for _, p := range c.r.Fn(c.fn).TypeParams {
		if c.r.typeParam(p).IsLifetime {
			return false
		}
	}
	return true
}

// An unbound lifetime variable settles to the function's sole lifetime, else the
// body's own lifetime of that name, else the declaring item's parameter p.
func (c *checker) lifetimeFallback(p EntityID) EntityID {
	if sole := c.r.soleLifetimeParam(c.fn); sole != 0 {
		return sole
	}
	if c.declaresNoLifetime() {
		return c.r.localLifetime(c.fn, c.r.Entities[p].Name)
	}
	return p
}

func (r *Result) localLifetime(fn EntityID, name string) EntityID {
	k := localLifetimeKey{fn, name}
	if id, ok := r.localLifetimes[k]; ok {
		return id
	}
	if r.localLifetimes == nil {
		r.localLifetimes = map[localLifetimeKey]EntityID{}
	}
	e := r.Entities[fn]
	id := r.newEntity(Entity{Kind: EntTypeParam, Name: name, Pkg: e.Pkg, File: e.File, Node: e.Node, Tok: e.Tok, Parent: fn, Vis: Visibility{Level: VisPub}})
	r.Entities[id].Detail = r.addTypeParam(TypeParamInfo{IsLifetime: true})
	r.Entities[id].Type = r.Types.Param(id)
	r.localLifetimes[k] = id
	return id
}

// Lifetime variables settle on their own, so they do not leave t's shape open.
func (v *varStore) onlyLifetimeVars(t TypeID) bool {
	t = v.resolve(t)
	if t == 0 || !v.r.Types.ContainsVar(t) {
		return true
	}
	n := v.r.Types.Node(t)
	if n.Kind == KVar {
		return v.lifetimeDefault[n.Var] != 0
	}
	for _, a := range n.Args {
		if !v.onlyLifetimeVars(a) {
			return false
		}
	}
	return v.onlyLifetimeVars(n.Elem)
}
