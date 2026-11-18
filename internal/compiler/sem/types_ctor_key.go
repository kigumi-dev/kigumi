package sem

import "encoding/hex"

// CtorKey spells the constructor of runtime type t without its arguments, from declarations and flags alone, never
// from an entity or type number: two builds that number them differently spell it the same.
func (r *Result) CtorKey(t TypeID) string {
	n := r.Types.nodes[t]
	head := itoa(int(n.Kind)) + ":" + itoa(int(n.Flags)) + ":"
	switch n.Kind {
	case KNamed, KIface, KClosure:
		return head + r.declKey(n.Ent)
	case KFn:
		if n.Var != 0 {
			return head + hex.EncodeToString([]byte(r.Types.escSets[n.Var-1]))
		}
	case KPrim, KUntyped:
		return head + r.TypeString(t)
	}
	return head
}

// declKey is ent's declaration identity, or where in its file a value with none (a method value) is written.
func (r *Result) declKey(ent EntityID) string {
	if id := r.DeclID(ent); !id.IsZero() {
		return id.String()
	}
	e := r.Entities[ent]
	if e.File == 0 {
		return r.entityName(ent)
	}
	return r.Tree(e.File).File.Name + "@" + itoa(int(e.Node))
}
