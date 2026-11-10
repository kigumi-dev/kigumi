package sem

const (
	aliasUnknown uint8 = iota
	aliasComputing
	aliasDone
)

// aliasType expands a transparent alias, detecting cycles. Predeclared
// aliases already carry their target.
func (r *Result) aliasType(id EntityID) TypeID {
	e := &r.Entities[id]
	if e.File == 0 {
		return e.Type
	}
	switch r.aliasState[id] {
	case aliasDone:
		return e.Type
	case aliasComputing:
		r.errAt(e.File, e.Node, cAliasCycle, e.Name)
		e.Type = TyPoison
		e.Flags |= EfPoison
		r.aliasState[id] = aliasDone
		return TyPoison
	}
	r.aliasState[id] = aliasComputing
	t := r.tree(e.File)
	body := typeDecl(t, e.Node).Body
	target := t.View(body).AsAliasBody().Type().ID()
	typ := r.resolveType(e.File, r.declScope(id), target, posAlias)
	if r.aliasState[id] == aliasComputing {
		r.aliasState[id] = aliasDone
		r.Entities[id].Type = typ
	}
	return r.Entities[id].Type
}

// checkFiniteSize rejects records and ADTs that contain themselves by value.
// Containers with indirection break the chain.
func (r *Result) checkFiniteSize(id EntityID) {
	info := r.typeDecl(id)
	if info.Size != sizeUnknown {
		return
	}
	// The walk memoizes answers; a top-level check owns the memo so it never outgrows one walk.
	if r.byValueDepth == 0 {
		r.byValueMemo = nil
		r.byValueDirty = 0
	}
	r.byValueDepth++
	defer func() { r.byValueDepth-- }()
	info.Size = sizeComputing
	infinite := false
	if info.Form == FormDefined && info.Underlying != 0 && r.containsByValue(info.Underlying, id) {
		infinite = true
	}
	for _, fld := range info.Fields {
		if r.containsByValue(r.Entities[fld].Type, id) {
			infinite = true
		}
	}
	for _, v := range info.Variants {
		for _, p := range r.variant(v).Payload {
			if r.containsByValue(p, id) {
				infinite = true
			}
		}
	}
	info = r.typeDecl(id)
	if infinite {
		info.Size = sizeInfinite
		e := &r.Entities[id]
		r.errAt(e.File, e.Node, cRecursiveType, e.Name)
		return
	}
	info.Size = sizeFinite
}

// byValueKey identifies a memoized "does t store target inline" query.
type byValueKey struct {
	t      TypeID
	target EntityID
}

// containsByValue reports whether t stores target inline, walking records,
// ADT payloads, and Option/Result but not heap containers: an ADT's payload sizes like a record field.
//
// The type table is a DAG, so `type P[T] = (T, T)` reached through `P[P[...]]` has two paths per level but one
// node: walking every path is exponential in the nesting depth. Answers are memoized per (type, target) so one
// walk costs a visit per node. An answer that consulted a type still being sized is provisional — that type's
// size changes when its own walk ends — so it is returned but not memoized.
func (r *Result) containsByValue(t TypeID, target EntityID) bool {
	key := byValueKey{t: t, target: target}
	if v, ok := r.byValueMemo[key]; ok {
		return v
	}
	r.finiteSizeSteps++
	mark := r.byValueDirty
	v := r.containsByValueNode(t, target)
	if r.byValueDirty == mark {
		if r.byValueMemo == nil {
			r.byValueMemo = map[byValueKey]bool{}
		}
		r.byValueMemo[key] = v
	}
	return v
}

// FiniteSizeSteps counts the containsByValue node evaluations a check spent sizing its types; a shared type DAG
// must cost one visit per node, not one per path.
func (r *Result) FiniteSizeSteps() int { return r.finiteSizeSteps }

func (r *Result) containsByValueNode(t TypeID, target EntityID) bool {
	n := r.Types.Node(t)
	if n.Kind != KNamed {
		return false
	}
	if n.Ent == r.Types.optionEnt || n.Ent == r.Types.resultEnt {
		for _, a := range n.Args {
			if r.containsByValue(a, target) {
				return true
			}
		}
		return false
	}
	if n.Ent == target {
		return true
	}
	info := r.typeDecl(n.Ent)
	if info.Size == sizeComputing {
		r.byValueDirty++
		return true
	}
	if info.Form == FormOpaque {
		return false
	}
	if info.Size == sizeUnknown {
		r.checkFiniteSize(n.Ent)
		info = r.typeDecl(n.Ent)
	}
	// An infinite type was already reported at its declaration; walking into it again would not terminate.
	if info.Size == sizeInfinite {
		return false
	}
	if info.Form == FormDefined {
		if info.Underlying == 0 || info.Underlying == TyPoison {
			return false
		}
		return r.containsByValue(r.Types.Subst(info.Underlying, instSubst(info.Params, n.Args)), target)
	}
	subst := map[EntityID]TypeID{}
	for i, p := range info.Params {
		if i < len(n.Args) {
			subst[p] = n.Args[i]
		}
	}
	for _, fld := range info.Fields {
		if r.containsByValue(r.Types.Subst(r.Entities[fld].Type, subst), target) {
			return true
		}
	}
	for _, v := range info.Variants {
		for _, p := range r.variant(v).Payload {
			if r.containsByValue(r.Types.Subst(p, subst), target) {
				return true
			}
		}
	}
	return false
}
