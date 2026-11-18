package sem

import "math/big"

func (tt *TypeTable) Named(ent EntityID, args []TypeID) TypeID {
	return tt.Intern(typeNode{Kind: KNamed, Ent: ent, Args: args})
}

func (tt *TypeTable) Option(t TypeID) TypeID { return tt.Named(tt.optionEnt, []TypeID{t}) }

func (tt *TypeTable) Result(t, e TypeID) TypeID { return tt.Named(tt.resultEnt, []TypeID{t, e}) }

func (tt *TypeTable) Iface(ent EntityID, args []TypeID) TypeID {
	return tt.Intern(typeNode{Kind: KIface, Ent: ent, Args: args})
}

func (tt *TypeTable) Param(ent EntityID) TypeID {
	return tt.Intern(typeNode{Kind: KParam, Ent: ent})
}

// RtType is the type of a value that holds a runtime type: a type node the backends intern.
func (tt *TypeTable) RtType() TypeID { return tt.Intern(typeNode{Kind: KRtType}) }

func (tt *TypeTable) IsRtType(t TypeID) bool { return t != 0 && tt.Kind(t) == KRtType }

func (tt *TypeTable) Closure(ent EntityID) TypeID {
	return tt.Intern(typeNode{Kind: KClosure, Ent: ent})
}

// ClosureInstance is the runtime identity of closure ent built for the instantiation args of its enclosing
// generic: a source closure is one type per instantiation, as its monomorphized copies are.
func (tt *TypeTable) ClosureInstance(ent EntityID, args []TypeID) TypeID {
	if len(args) == 0 {
		return tt.Closure(ent)
	}
	return tt.Intern(typeNode{Kind: KClosure, Ent: ent, Args: args})
}

func (tt *TypeTable) Var(n uint32) TypeID { return tt.Intern(typeNode{Kind: KVar, Var: n}) }

// ConstVal interns a const generic argument value: ty is its
// declared type (usize, a fixed-width integer, Int or Bool).
func (tt *TypeTable) ConstVal(v int64, ty TypeID) TypeID {
	return tt.ConstValBig(big.NewInt(v), ty)
}

// ConstValBig is ConstVal for a value wider than 64 bits, which an i128 or
// u128 const parameter argument can carry.
func (tt *TypeTable) ConstValBig(v *big.Int, ty TypeID) TypeID {
	key := v.String()
	idx, ok := tt.constIndex[key]
	if !ok {
		idx = uint32(len(tt.constVals))
		tt.constVals = append(tt.constVals, new(big.Int).Set(v))
		tt.constIndex[key] = idx
	}
	return tt.Intern(typeNode{Kind: KConst, Var: idx, Elem: ty})
}

// ConstValue reads back a KConst type's value and declared type, cut to the
// low 64 bits for callers that only carry a width or a length; ConstValueBig
// is the whole value.
func (tt *TypeTable) ConstValue(t TypeID) (int64, TypeID) {
	n := tt.nodes[t]
	v := tt.constVals[n.Var]
	return v.Int64(), n.Elem
}

// ConstValueBig reads back a KConst type's value at full width and its
// declared type.
func (tt *TypeTable) ConstValueBig(t TypeID) (*big.Int, TypeID) {
	n := tt.nodes[t]
	return new(big.Int).Set(tt.constVals[n.Var]), n.Elem
}

// IsConst reports whether t is a const generic argument value.
func (tt *TypeTable) IsConst(t TypeID) bool { return t != 0 && tt.nodes[t].Kind == KConst }

// Rebuild is the named, interface or fn type t with args and elem in place of its own.
func (tt *TypeTable) Rebuild(t TypeID, args []TypeID, elem TypeID) TypeID {
	n := tt.nodes[t]
	return tt.Intern(tt.absorbEscMarks(typeNode{Kind: n.Kind, Flags: n.Flags, Ent: n.Ent, Elem: elem, Lifetime: n.Lifetime, Var: n.Var, Args: args}))
}

// RebuildRef is the reference or pointer type t over elem, without a lifetime.
func (tt *TypeTable) RebuildRef(t TypeID, elem TypeID) TypeID {
	n := tt.nodes[t]
	return tt.Intern(typeNode{Kind: n.Kind, Flags: n.Flags, Elem: elem})
}
