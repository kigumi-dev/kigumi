package sem

import (
	"math/big"

	"kigumi/internal/syntax"
)

type TypeDeclInfo struct {
	Form     TypeForm
	Metadata []MetaRef
	Params   []EntityID
	Fields   []EntityID
	Variants []EntityID
	Impls    []ImplAssert
	Layout   string
	// Align is layout(C, align: N)'s N, 0 when unset.
	Align   int64
	Members map[string]OverloadSetID
	Ops     map[string]OverloadSetID
	Drop    EntityID
	Home    PackageID
	Size    sizeState
	// Guarantee is the resolved `@unsafeGuarantee` contract, nil when the
	// type carries none (resource/opaque properties then default to false).
	Guarantee *UnsafeGuarantee
	// Underlying is a FormDefined type's direct underlying type (unsubstituted:
	// its own type parameters stand for the generic case), TyPoison on a cycle.
	Underlying TypeID
	// Schema marks the type `@schema`: a function returning it is a schema,
	// not any type merely named the same.
	Schema           bool
	SchemaRepeatable bool
	SchemaOrdered    bool
	SchemaTarget     string
}

type FnInfo struct {
	Recv       RecvKind
	Metadata   []MetaRef
	Owner      EntityID
	TypeParams []EntityID
	Params     []EntityID
	SelfParam  EntityID
	Sig        TypeID
	Declared   Effects
	Summary    EffectSummary
	Ops        OpSummary
	// Mobility and BoundBy classify an async fn's Future once the frame
	// layout is known; BoundBy names the first type that pins it.
	Mobility Mobility
	BoundBy  string
	Edges    []EffectEdge
	Body     syntax.NodeID
	Abi      string
	// Export names the C ABI a function with a body is exported under.
	Export string
	// Naked marks an export(C, naked) function: its body is one asm block
	// emitted as module-level assembly, and its value is a C pointer.
	Naked bool
	Set   OverloadSetID
	// RecvParams are the method's own copies of the owner's type
	// parameters when the receiver binders carry bounds (`Array[T: Eq]`).
	RecvParams []EntityID
	// Witnesses are the hidden trailing parameters carrying the interface
	// witnesses of constrained type parameters (design: witness passing).
	Witnesses []WitnessSlot
	// ConstParams are the hidden trailing parameters carrying the runtime
	// value of every const generic parameter, own or inherited from the
	// receiver's type: erased type arguments carry no such value,
	// so each becomes an ordinary implicit argument after Witnesses.
	ConstParams []EntityID
	// CalledParams are the fn-typed parameters the body calls; callers add
	// the effects of the matching arguments.
	CalledParams []EntityID
	// CalledFieldParams are `pure?` fields read through a parameter
	// (`app.view(...)`); callers add the argument's field effect.
	CalledFieldParams []FieldParamKey
	// CalledWitnessReqs are `pure?` interface requirements read through one
	// of the function's own type parameters (`t.view(x)` where `t: T`);
	// callers add the effect of the resolved witness for their type
	// argument.
	CalledWitnessReqs []WitnessReqKey
	// EffectPoly marks a `pure? fn` interface requirement.
	EffectPoly bool
	// RetLifetime is the lifetime parameter entity the return type's `'a`
	// tag names (explicit or elided), 0 when the return carries none.
	RetLifetime EntityID
	// LinkName is `@linkName`'s override of an extern/export fn's external
	// symbol, empty to use the source name.
	LinkName string
	// Section is `@section`'s object-file section for an owned fn's body.
	Section string
	// CtxDirect is the set of keys this function's body reads directly
	// unbound; CtxDirectRefs is each occurrence's use-site node.
	CtxDirect     []EntityID
	CtxDirectRefs []ctxNodeRef
	// CtxDirectReal marks a CtxDirect key with a real value use, as opposed
	// to only print's phantom dependency.
	CtxDirectReal map[EntityID]bool
	// CtxReal is CtxDirectReal transitively closed over the call graph.
	CtxReal map[EntityID]bool
	// ctxCalls are this function's deferred call-site context requirements.
	ctxCalls []ctxCallEdge
	// CtxRequires is the final, sorted set of context keys this function
	// requires from its caller, after the call-graph fixpoint.
	CtxRequires []EntityID
	// CtxParam maps a required key to the hidden parameter entity reading it.
	CtxParam map[EntityID]EntityID
	// DefaultOf is the declaration a hidden default thunk computes an argument for; DefaultParam names that parameter.
	DefaultOf EntityID
	// InitOf is the const or static this hidden function initializes.
	InitOf EntityID
	// DefaultTry marks a thunk that returns `T!` because its default expression uses `?`.
	DefaultTry   bool
	DefaultParam string
}

type InterfaceInfo struct {
	Params        []EntityID
	Reqs          []EntityID
	SelfParam     EntityID
	ObjectSafe    int8
	ObjectSafeWhy string
	// Derivable marks the interface `@derivable`, so its single requirement
	// may be generated without compiler source naming the interface.
	Derivable bool
}

type constKind uint8

const (
	constNone constKind = iota
	constInt
	constFloat
	constString
	constBytes
	constBool
	constChar
	constRecord
)

// constValue holds a compile-time value; exactly one field is set per Kind.
type constValue struct {
	Kind constKind
	Int  *big.Int
	// Typ is the fixed-width integer type a typed const carries through const expressions; 0 for an untyped value.
	Typ TypeID
	// Shift is the widest untyped shift count in the expression; a target type must be wider than it.
	Shift int
	Float *big.Float
	Str   string
	Bool  bool
	Char  rune
	// Fields are in declaration order.
	Fields []constValue
}

const (
	constUneval uint8 = iota
	constEvaluating
	constDone
	constFailed
)

type ConstInfo struct {
	Value    constValue
	State    uint8
	Metadata []MetaRef
	// Thunk computes a `comptime` initializer; the value exists only after folding.
	Thunk EntityID
}

type LocalInfo struct {
	Scope    ScopeID
	Place    PlaceID
	Metadata []MetaRef
	// Lifetime is the parameter entity a parameter's own `&'a T` tag names,
	// 0 for an untagged reference or a non-reference parameter.
	Lifetime EntityID
}
