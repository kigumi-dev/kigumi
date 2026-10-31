package ops

import (
	"fmt"
	"strings"
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

func (c OpClass) Set() ClassSet { return classBit(c) }
