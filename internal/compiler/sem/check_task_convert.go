package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// taskConversion is a value of type from converted to interface iface in the body of owner.
type taskConversion struct {
	owner EntityID
	from  TypeID
	iface TypeID
	f     FileID
	n     syntax.NodeID
}

// taskImpl is a type a value of an interface may be: one converted to it, or a generic conversion's type under a
// binding some call makes; open when the bindings ran out before every type parameter was bound.
type taskImpl struct {
	t     TypeID
	conv  taskConversion
	bound bool
	open  bool
}

func (r *Result) noteTaskConversions(owner EntityID, f FileID, n syntax.NodeID, co Coercion) {
	from := co.From
	for _, st := range co.Steps {
		if st.Kind == CoExistential && r.Types.Kind(st.To) == KIface {
			r.taskConversions = append(r.taskConversions, taskConversion{r.bodyOwner(owner), from, st.To, f, n})
		}
		from = st.To
	}
}

func (r *Result) bodyOwner(e EntityID) EntityID {
	for e != 0 && r.Entities[e].Kind == EntClosure {
		e = r.Entities[e].Parent
	}
	return e
}

// ifaceImplementers are the types converted to interface ent: as written, or, for a conversion in a generic body,
// under each binding of its type parameters the calls and fn values reaching it make, and left open past what is known.
func (r *Result) ifaceImplementers(ent EntityID) []taskImpl {
	if out, ok := r.taskImplementers[ent]; ok {
		return out
	}
	var out []taskImpl
	for _, cv := range r.taskConversions {
		if r.Types.Node(cv.iface).Ent != ent {
			continue
		}
		if !r.OpenType(cv.from) {
			out = append(out, taskImpl{t: cv.from, conv: cv})
			continue
		}
		substs, cut := r.instSubsts(cv.owner, 4)
		for _, s := range substs {
			t := r.Types.Subst(cv.from, s)
			out = append(out, taskImpl{t: t, conv: cv, bound: true, open: r.OpenType(t)})
		}
		if len(substs) == 0 || cut {
			out = append(out, taskImpl{t: cv.from, conv: cv, open: true})
		}
	}
	if r.seal.witnesses != nil {
		if r.taskImplementers == nil {
			r.taskImplementers = map[EntityID][]taskImpl{}
		}
		r.taskImplementers[ent] = out
	}
	return out
}

// OpenType reports a type parameter (not a lifetime) or an inference variable in t, which some binding may make a Task.
func (r *Result) OpenType(t TypeID) bool {
	seen := map[TypeID]bool{}
	var walk func(TypeID) bool
	walk = func(t TypeID) bool {
		if t == 0 || t == TyPoison || seen[t] {
			return false
		}
		seen[t] = true
		n := r.Types.Node(t)
		if n.Kind == KVar || n.Kind == KParam && !r.typeParam(n.Ent).IsLifetime {
			return true
		}
		return walk(n.Elem) || slices.ContainsFunc(n.Args, walk)
	}
	return walk(t)
}
