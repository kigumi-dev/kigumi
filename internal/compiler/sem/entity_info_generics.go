package sem

import "kigumi/internal/compiler/syntax"

type TypeParamInfo struct {
	Index       int
	Constraints []Constraint
	// IsConst marks a `const N: usize` parameter; ConstType is its
	// declared value type.
	IsConst   bool
	ConstType TypeID
	// IsLifetime marks a `'a` parameter: its Type is still a
	// KParam like an ordinary type parameter, matched by entity identity.
	IsLifetime bool
	// Opened marks a parameter a type test binds to an argument it reads from the box; Local is the hidden
	// local holding it, a runtime type or, for a const parameter, its value.
	Opened bool
	Local  EntityID
}

type ConstraintKind uint8

const (
	CIface ConstraintKind = iota
	CCopy
	CSend
	CSync
	CFn
	CFnMut
	CFnOnce
)

type Constraint struct {
	Kind ConstraintKind
	Type TypeID
	Node syntax.NodeID
}

type OverloadSet struct {
	Name    string
	Pkg     PackageID
	Owner   EntityID
	Members []EntityID
}
