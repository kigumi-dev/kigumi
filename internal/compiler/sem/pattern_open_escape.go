package sem

import "kigumi/internal/compiler/syntax"

// keepOpened rejects a result of type t, produced at node n, that names a parameter opened since mark: each
// test opens its own type, so a value of it past the test would stand for whatever another run opened.
func (c *checker) keepOpened(n syntax.NodeID, mark int, t TypeID) TypeID {
	if mark == len(c.opens) {
		return t
	}
	t = c.vars.resolve(t)
	opened := map[EntityID]bool{}
	for _, p := range c.opens[mark:] {
		opened[p] = true
	}
	if c.r.namesAny(t, opened, map[EntityID]bool{}) {
		c.errAt(n, cOpenedEscape, t)
		return TyPoison
	}
	return t
}

// namesAny reports whether t names one of params, through the captures and signature of a closure it names.
func (r *Result) namesAny(t TypeID, params, seen map[EntityID]bool) bool {
	if t == 0 {
		return false
	}
	n := r.Types.Node(t)
	switch n.Kind {
	case KParam:
		return params[n.Ent]
	case KClosure:
		if seen[n.Ent] || r.Entities[n.Ent].Kind != EntClosure {
			break
		}
		seen[n.Ent] = true
		info := r.closure(n.Ent)
		for _, cp := range info.Captures {
			if r.namesAny(r.Entities[cp.Local].Type, params, seen) {
				return true
			}
		}
		if r.namesAny(info.Sig, params, seen) {
			return true
		}
	}
	if r.namesAny(n.Elem, params, seen) {
		return true
	}
	for _, a := range n.Args {
		if r.namesAny(a, params, seen) {
			return true
		}
	}
	return false
}
