package instance

import "kigumi/internal/compiler/sem"

// Test is how a type test compares a box's dynamic type with the type it names.
type Test uint8

const (
	// TestExact matches one runtime type, composed from the body's environment where it names parameters.
	TestExact Test = iota
	// TestAny matches every instantiation of the entity: each argument is a parameter the test opens.
	TestAny
)

// TypeTest classifies the pattern type pat of a type test.
func TypeTest(r *sem.Result, pat sem.TypeID) Test {
	n := r.Types.Node(pat)
	if n.Kind != sem.KNamed {
		return TestExact
	}
	open := false
	for _, a := range n.Args {
		an := r.Types.Node(a)
		if an.Kind != sem.KParam {
			return TestExact
		}
		if _, ok := r.OpenedLocal(an.Ent); ok {
			open = true
		} else if !r.IsLifetimeParam(an.Ent) {
			return TestExact
		}
	}
	if open {
		return TestAny
	}
	return TestExact
}
