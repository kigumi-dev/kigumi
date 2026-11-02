package sem

import "slices"

// holdsArg reports whether named type ent holds its type argument i only as whole parts: a field or payload of exactly
// that type, or such an argument of a type holding it the same way. Any other use, an interface, or a cycle is invariant.
func (r *Result) holdsArg(ent EntityID, i int, seen map[EntityID]bool) bool {
	if r.Entities[ent].Kind != EntType || seen[ent] {
		return false
	}
	seen[ent] = true
	defer delete(seen, ent)
	decl := r.typeDecl(ent)
	if i >= len(decl.Params) {
		return false
	}
	p := r.Types.Param(decl.Params[i])
	var parts []TypeID
	for _, f := range decl.Fields {
		parts = append(parts, r.Entities[f].Type)
	}
	for _, v := range decl.Variants {
		parts = append(parts, r.variant(v).Payload...)
	}
	if decl.Underlying != 0 {
		parts = append(parts, decl.Underlying)
	}
	return !slices.ContainsFunc(parts, func(t TypeID) bool { return !r.heldAs(t, p, seen) })
}

func (r *Result) heldAs(t, p TypeID, seen map[EntityID]bool) bool {
	n := r.Types.Node(t)
	switch {
	case t == p || !r.Types.Contains(t, p):
		return true
	case n.Kind == KNamed && r.holdsElements(t):
		for i, a := range n.Args {
			if a != p && r.Types.Contains(a, p) || a == p && !r.holdsArg(n.Ent, i, seen) {
				return false
			}
		}
		return true
	}
	return false
}

// holdsElements reports whether a value of named type t is its own storage, copied or moved whole: a record, a variant,
// a defined type over such, or the opaque Array and FixedArray, whose elements MIR reads off their constructors.
func (r *Result) holdsElements(t TypeID) bool {
	n := r.Types.Node(t)
	if n.Kind != KNamed || r.Entities[n.Ent].Kind != EntType {
		return n.Kind != KIface
	}
	switch decl := r.typeDecl(n.Ent); decl.Form {
	case FormOpaque:
		return n.Ent == r.Types.arrayEnt && n.Ent != 0 || r.isFixedArray(t)
	case FormDefined:
		return decl.Underlying != t && r.holdsElements(decl.Underlying)
	}
	return true
}
