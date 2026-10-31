package mir

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/sem"
)

// OpClass is how a runtime primitive reads one argument or result: the value
// shape the C and VM implementations assume, not the full Kigumi type.
type OpClass uint8

const (
	ClassUnit OpClass = iota
	ClassBool
	ClassChar
	ClassInt
	ClassFloat
	ClassString
	ClassBytes
	ClassArray
	ClassOption
	ClassResult
	ClassFn
	ClassPtr
	ClassNamed
	classCount
)

var opClassNames = [classCount]string{"unit", "bool", "char", "int", "float", "string", "bytes", "array", "option", "result", "fn", "ptr", "named"}

func (c OpClass) String() string { return opClassNames[c] }

// ClassSet is a set of OpClass; the zero value is the empty set, ClassAnyOf is every class.
type ClassSet uint16

const ClassAnyOf ClassSet = 1<<classCount - 1

func classBit(c OpClass) ClassSet { return 1 << c }

func (s ClassSet) Has(c OpClass) bool { return s&classBit(c) != 0 }

func (s ClassSet) String() string {
	if s == ClassAnyOf {
		return "any"
	}
	var parts []string
	for c := OpClass(0); c < classCount; c++ {
		if s.Has(c) {
			parts = append(parts, c.String())
		}
	}
	return strings.Join(parts, "|")
}

func parseClassSet(text string) (ClassSet, error) {
	if text == "any" {
		return ClassAnyOf, nil
	}
	var set ClassSet
	for _, part := range strings.Split(text, "|") {
		found := false
		for c := OpClass(0); c < classCount; c++ {
			if opClassNames[c] == part {
				set |= classBit(c)
				found = true
			}
		}
		if !found {
			return 0, fmt.Errorf("unknown class %q", part)
		}
	}
	return set, nil
}

// StdOpDesc is the registry's type descriptor for one op: the classes of its
// arguments (the receiver first) and of its result.
type StdOpDesc struct {
	Args   []ClassSet
	Result ClassSet
}

// opClassOf is the class of t, or false for a type the verifier cannot pin
// down (a type parameter, a still-open variable, an interface value).
func (p *Program) opClassOf(t sem.TypeID) (OpClass, bool) {
	tt := p.R.Types
	t = p.R.UnderlyingRepr(p.derefAll(t))
	switch t {
	case 0, sem.TyNever, sem.TyPoison:
		return 0, false
	case sem.TyUnit:
		return ClassUnit, true
	case sem.TyBool:
		return ClassBool, true
	case sem.TyChar:
		return ClassChar, true
	case sem.TyString:
		return ClassString, true
	case sem.TyBytes:
		return ClassBytes, true
	case sem.TyUntypedInt:
		return ClassInt, true
	case sem.TyUntypedFloat:
		return ClassFloat, true
	}
	switch tt.Kind(t) {
	case sem.KFn, sem.KClosure:
		return ClassFn, true
	case sem.KPtr:
		return ClassPtr, true
	case sem.KPrim:
		switch {
		case tt.IsInteger(t):
			return ClassInt, true
		case tt.IsFloat(t):
			return ClassFloat, true
		}
	case sem.KNamed:
		return p.namedOpClass(t), true
	}
	return 0, false
}

func (p *Program) namedOpClass(t sem.TypeID) OpClass {
	tt := p.R.Types
	if _, ok := tt.IsOption(t); ok {
		return ClassOption
	}
	if _, _, ok := tt.IsResult(t); ok {
		return ClassResult
	}
	if tt.Node(t).Ent == tt.ArrayEnt() || p.R.IsFixedArray(t) {
		return ClassArray
	}
	return ClassNamed
}

func (p *Program) opClassMismatch(want ClassSet, got sem.TypeID) string {
	c, ok := p.opClassOf(got)
	if !ok || want.Has(c) {
		return ""
	}
	return "is a " + c.String() + " (" + p.R.TypeString(got) + "), the runtime reads " + want.String()
}
