package verify

import (
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// opClassOf is the class of t, or false for a type the verifier cannot pin
// down (a type parameter, a still-open variable, an interface value).
func (p *verifier) opClassOf(t sem.TypeID) (opdef.OpClass, bool) {
	tt := p.R.Types
	t = p.R.UnderlyingRepr(p.DerefAll(t))
	switch t {
	case 0, sem.TyNever, sem.TyPoison:
		return 0, false
	case sem.TyUnit:
		return opdef.ClassUnit, true
	case sem.TyBool:
		return opdef.ClassBool, true
	case sem.TyChar:
		return opdef.ClassChar, true
	case sem.TyString:
		return opdef.ClassString, true
	case sem.TyBytes:
		return opdef.ClassBytes, true
	case sem.TyUntypedInt:
		return opdef.ClassInt, true
	case sem.TyUntypedFloat:
		return opdef.ClassFloat, true
	}
	switch tt.Kind(t) {
	case sem.KFn, sem.KClosure:
		return opdef.ClassFn, true
	case sem.KPtr:
		return opdef.ClassPtr, true
	case sem.KPrim:
		switch {
		case tt.IsInteger(t):
			return opdef.ClassInt, true
		case tt.IsFloat(t):
			return opdef.ClassFloat, true
		}
	case sem.KNamed:
		return p.namedOpClass(t), true
	}
	return 0, false
}

func (p *verifier) namedOpClass(t sem.TypeID) opdef.OpClass {
	tt := p.R.Types
	if _, ok := tt.IsOption(t); ok {
		return opdef.ClassOption
	}
	if _, _, ok := tt.IsResult(t); ok {
		return opdef.ClassResult
	}
	if tt.Node(t).Ent == tt.ArrayEnt() || p.R.IsFixedArray(t) {
		return opdef.ClassArray
	}
	return opdef.ClassNamed
}

func (p *verifier) opClassMismatch(want opdef.ClassSet, got sem.TypeID) string {
	c, ok := p.opClassOf(got)
	if !ok || want.Has(c) {
		return ""
	}
	return "is a " + c.String() + " (" + p.R.TypeString(got) + "), the runtime reads " + want.String()
}
