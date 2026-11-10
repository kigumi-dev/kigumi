package schema

// TypeRefKind discriminates TypeRef's structured shape; a consumer
// switches on this, never on Display.
type TypeRefKind string

const (
	TRPoison  TypeRefKind = "poison"
	TRPrim    TypeRefKind = "prim"
	TRNamed   TypeRefKind = "named"
	TRIface   TypeRefKind = "iface"
	TRParam   TypeRefKind = "param"
	TRFn      TypeRefKind = "fn"
	TRClosure TypeRefKind = "closure"
	TRBorrow  TypeRefKind = "borrow"
	TRPointer TypeRefKind = "pointer"
	TRConst   TypeRefKind = "const"
)

// WellKnown flags a TRNamed type the IR recognizes structurally, so a
// consumer never pattern-matches a display name for it.
type WellKnown string

const (
	WKNone   WellKnown = ""
	WKOption WellKnown = "option"
	WKResult WellKnown = "result"
	WKArray  WellKnown = "array"
	// WKFixedArray is `FixedArray[T, N]`: Args[0] the element type,
	// Args[1] a TRConst carrying N in Value.
	WKFixedArray WellKnown = "fixedArray"
	WKFuture     WellKnown = "future"
	WKError      WellKnown = "error"
)

// TypeRef is one node of the structured type graph; Display is for logs
// and diagnostics only, never re-parsed for structure.
type TypeRef struct {
	Kind      TypeRefKind `json:"kind"`
	Display   string      `json:"display"`
	Prim      string      `json:"prim,omitempty"`
	Decl      *DeclRef    `json:"decl,omitempty"`
	WellKnown WellKnown   `json:"wellKnown,omitempty"`
	Args      []*TypeRef  `json:"args,omitempty"`
	Elem      *TypeRef    `json:"elem,omitempty"`
	Params    []*TypeRef  `json:"params,omitempty"`
	Mutable   bool        `json:"mutable,omitempty"`
	Lifetime  *TypeRef    `json:"lifetime,omitempty"`
	// Value is a TRConst node's literal value in decimal; Display is not
	// re-parsed for it.
	Value string `json:"value,omitempty"`
	// Variadic marks a C `...` signature (TRFn).
	Variadic bool `json:"variadic,omitempty"`
}

// DeclRef names a declaration by stable ID (id.go), never a process-local
// ordinal.
type DeclRef struct {
	ID      string `json:"id"`
	Package string `json:"package"`
	Name    string `json:"name"`
}
