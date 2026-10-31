package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// containerShape names the places inside a value the fn tracker follows: a tuple's components, an Option's payload, an Array's elements.
// An Array has one field standing for every element.
func (r *Result) containerShape(t TypeID) (fields []EntityID, elems []TypeID, ok bool) {
	tt := r.Types
	n := tt.Node(t)
	if n.Kind != KNamed || len(n.Args) == 0 {
		return nil, nil, false
	}
	switch {
	case tt.isTupleEnt(n.Ent):
		return r.typeDecl(n.Ent).Fields, n.Args, true
	case n.Ent == tt.optionEnt:
		return []EntityID{r.containerField(fieldPayload)}, n.Args[:1], true
	case n.Ent == tt.resultEnt && tt.resultEnt != 0 && len(n.Args) == 2:
		return []EntityID{r.containerField(fieldOk), r.containerField(fieldErr)}, n.Args[:2], true
	case n.Ent == tt.arrayEnt && tt.arrayEnt != 0, r.isFixedArray(t):
		return []EntityID{r.containerField(fieldElement)}, n.Args[:1], true
	}
	return nil, nil, false
}

const (
	fieldPayload = iota
	fieldElement
	fieldOk
	fieldErr
)

var containerFieldNames = [4]string{"payload", "element", "ok", "err"}

func (r *Result) containerField(i int) EntityID {
	if r.containerFields[i] == 0 {
		r.containerFields[i] = r.newEntity(Entity{Kind: EntFieldParam, Name: containerFieldNames[i], Type: TyUnit})
	}
	return r.containerFields[i]
}

func (r *Result) placeShape(t TypeID) ([]EntityID, []TypeID) {
	if fields, elems, ok := r.containerShape(t); ok {
		return fields, elems
	}
	fields := r.recordFieldsOf(t)
	types := make([]TypeID, len(fields))
	for i, f := range fields {
		types[i] = r.Entities[f].Type
	}
	return fields, types
}

func (r *Result) holdsFnPlace(t TypeID) bool { return r.holdsFn(t, 0) || r.holdsFnElem(t) }

func (r *Result) holdsFnElem(t TypeID) bool {
	_, elems, ok := r.containerShape(t)
	for _, e := range elems {
		if ok && (r.holdsFn(e, 0) || r.holdsFnElem(e)) {
			return true
		}
	}
	return false
}

func (r *Result) containerLitEnt(file FileID, expr syntax.NodeID) EntityID {
	key := literalEntKey{File: file, Node: expr}
	if id, ok := r.containerEnts[key]; ok {
		return id
	}
	id := r.newEntity(Entity{Kind: EntFieldParam, Name: "<container>", Pkg: r.Files[file].Pkg, File: file, Node: expr})
	if r.containerEnts == nil {
		r.containerEnts = map[literalEntKey]EntityID{}
		r.containerLits = map[EntityID]bool{}
	}
	r.containerEnts[key] = id
	r.containerLits[id] = true
	return id
}

func (r *Result) containerLitElems(file FileID, expr syntax.NodeID) ([]syntax.NodeID, bool) {
	t := r.tree(file)
	switch t.Kind(expr) {
	case syntax.TupleLit:
		return slices.Collect(t.List(expr).IDs()), true
	case syntax.CallExpr:
		call, ok := r.Files[file].Calls[expr]
		if !ok || call.Plan == nil {
			return nil, false
		}
		switch e := &r.Entities[call.Callee]; {
		case call.Kind == CallVariant && e.Kind == EntVariant && (e.Parent == r.Types.optionEnt && e.Name == "Some" || e.Parent == r.Types.resultEnt && r.Types.resultEnt != 0 && (e.Name == "Ok" || e.Name == "Err")):
			return call.Plan.Eval, true
		case call.Kind == CallAssoc && e.Kind == EntFn && e.Name == "of" && r.Fn(call.Callee).Owner == r.Types.arrayEnt && r.Types.arrayEnt != 0:
			return call.Plan.Eval, true
		case call.Kind == CallAssoc && r.fixedFilled(call.Callee):
			return call.Plan.Eval[:min(1, len(call.Plan.Eval))], true
		}
	}
	return nil, false
}

// The tracker reads the literal form of `FixedArray.filled` instead of its body.
func (r *Result) fixedFilled(fn EntityID) bool {
	return r.Entities[fn].Kind == EntFn && r.Entities[fn].Name == "filled" && r.Fn(fn).Owner == r.langItem(0, 0, "FixedArray")
}

// `Err(e)` fills the second component of a Result, not the first.
func (r *Result) containerLitSlot(file FileID, lit syntax.NodeID, i, fields int) int {
	if call, ok := r.Files[file].Calls[lit]; ok && call.Kind == CallVariant && r.Entities[call.Callee].Name == "Err" && fields == 2 {
		return 1
	}
	return min(i, fields-1)
}
