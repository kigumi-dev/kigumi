package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// Merged after every body is checked, since a copy taken at the let would miss later writes to src.
type fnUnion struct {
	dst, src EntityID
	typ      TypeID
	file     FileID
	node     syntax.NodeID
}

func (r *Result) concretePlace(ent EntityID) bool {
	root := r.rootOf(ent)
	return r.Entities[root].Kind == EntLocal || r.isResultEnt(root)
}

func (c *checker) unionInto(dst, src EntityID, t TypeID, node syntax.NodeID) {
	t = c.vars.resolve(t)
	if r := c.r; r.Types.Kind(t) == KRef {
		t = r.Types.Node(t).Elem
	}
	if dst == src || len(c.r.recordFieldsOf(t)) == 0 || !c.r.holdsFn(t, 0) {
		return
	}
	c.r.fnUnions = append(c.r.fnUnions, fnUnion{dst: dst, src: src, typ: t, file: c.f, node: node})
}

func (r *Result) fnAliasOf(ent EntityID) EntityID {
	for range maxNestedFnDepth * 2 {
		next, ok := r.fnCopyAlias[ent]
		if !ok {
			break
		}
		ent = next
	}
	return ent
}

func (r *Result) applyUnions() bool {
	changed := false
	for _, u := range r.fnUnions {
		if r.mergePlace(u.dst, r.fnAliasOf(u.src), u.typ, u, 0) {
			changed = true
		}
	}
	return changed
}

func (r *Result) mergePlace(dst, src EntityID, t TypeID, u fnUnion, depth int) bool {
	if depth >= maxNestedFnDepth || dst == src {
		return false
	}
	changed := false
	for _, f := range r.recordFieldsOf(t) {
		ft := r.Entities[f].Type
		from, to := fieldFnKey{Ent: src, Field: f}, fieldFnKey{Ent: dst, Field: f}
		if r.fnTypedType(ft) {
			if r.mergeFnField(from, to, u) {
				changed = true
			}
			continue
		}
		if r.holdsFn(ft, 0) && r.mergePlace(r.nestedFnOwner(to), r.fnAliasOf(r.nestedFnOwner(from)), ft, u, depth+1) {
			changed = true
		}
	}
	return changed
}

func (r *Result) mergeFnField(from, to fieldFnKey, u fnUnion) bool {
	if r.fieldWitnessPoisoned[to] {
		return false
	}
	if r.fieldWitnessPoisoned[from] {
		if at, ok := r.fieldPoisonSite[from]; ok {
			return r.poisonFieldKeyAt(to, at.File, at.Node)
		}
		return r.poisonFieldKeyAt(to, u.file, u.node)
	}
	changed := r.joinKnownFieldFnAt(to, r.knownFieldFn[from], fnSiteSrc{from: from, hasFrom: true, file: u.file, node: u.node})
	fromEnt, tracked := r.fieldParamEnts[from]
	if !tracked {
		return changed
	}
	toEnt := r.fieldParamEnt(to)
	if w := r.localWitness[fromEnt]; len(w) > 0 && r.joinLocalWitness(toEnt, w) {
		changed = true
	}
	for _, fn := range slices.Clone(r.fieldBindOwners[fromEnt]) {
		for _, e := range slices.Clone(r.Fn(fn).Edges) {
			if e.Param == fromEnt && (e.Kind == EdgeFieldBind || e.Kind == EdgeIfaceArg) && r.addFieldBindEdge(fn, e.Kind, e.Target, to, e.File, e.Node) {
				changed = true
			}
		}
	}
	return changed
}
