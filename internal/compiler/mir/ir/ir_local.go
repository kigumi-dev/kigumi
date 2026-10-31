package ir

import "kigumi/internal/compiler/sem"

type Local struct {
	Name string
	Type sem.TypeID
	Ent  sem.EntityID
	Cell bool
	// Borrowed marks a `self` / `mut self` receiver: an alias of the
	// caller's value that the method neither owns nor may move out.
	Borrowed bool
	// Cross marks a local PlanDirect made for one crossing: the crossing
	// owns what it holds, the ownership plan having run before it existed.
	Cross bool
	// FullExpr numbers the full expression that created a temporary (0: none);
	// one holding a resource is released at that expression's end.
	FullExpr int32
}
