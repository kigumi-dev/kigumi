package sem

import (
	"kigumi/internal/diag"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// FileInfo is the side table of one syntax.Tree; dense slices are indexed by NodeID.
type FileInfo struct {
	Tree   *syntax.Tree
	Pkg    PackageID
	Types  []TypeID
	Uses   []EntityID
	Defs   []EntityID
	Scopes []ScopeID

	Coerce map[syntax.NodeID]Coercion
	// CoerceWitness is an existential coercion's resolved witnesses, keyed
	// by the same node as Coerce; root-bridge reachability reads it.
	CoerceWitness map[syntax.NodeID][]EntityID
	Calls         map[syntax.NodeID]CallInfo
	Pats          map[syntax.NodeID]PatInfo
	Matches       map[syntax.NodeID]MatchInfo
	Places        map[syntax.NodeID]PlaceID
	Narrow        map[syntax.NodeID][]NarrowFact
	Literals      map[syntax.NodeID]constValue
	Asm           map[syntax.NodeID]*AsmInfo
	OptChains     map[syntax.NodeID]*OptChain
	ChainHoles    map[syntax.NodeID]EntityID
	Diags         []diag.Diagnostic
	// CtxUse resolves a direct context-key value reference.
	CtxUse map[syntax.NodeID]CtxBind
	// CtxCall resolves one call's hidden provider arguments, in the
	// callee's CtxRequires order.
	CtxCall map[syntax.NodeID][]CtxBind
	// CtxProviders maps a `context (...) { }` node to the hidden local
	// holding each provider expression's one-time evaluated value.
	CtxProviders map[syntax.NodeID][]EntityID
	// Defaults maps a DefaultArg node to the hidden thunk computing the omitted argument.
	Defaults map[syntax.NodeID]EntityID
	// Imported maps each package this file imports to its first import entity.
	Imported map[PackageID]EntityID
}

func newFileInfo(t *syntax.Tree, pkg PackageID) FileInfo {
	n := t.NodeCount()
	return FileInfo{
		Tree: t, Pkg: pkg,
		Types: make([]TypeID, n), Uses: make([]EntityID, n), Defs: make([]EntityID, n), Scopes: make([]ScopeID, n),
		Coerce: map[syntax.NodeID]Coercion{}, CoerceWitness: map[syntax.NodeID][]EntityID{},
		Calls: map[syntax.NodeID]CallInfo{}, Pats: map[syntax.NodeID]PatInfo{},
		Matches: map[syntax.NodeID]MatchInfo{}, Places: map[syntax.NodeID]PlaceID{},
		Narrow: map[syntax.NodeID][]NarrowFact{}, Literals: map[syntax.NodeID]constValue{},
		Asm: map[syntax.NodeID]*AsmInfo{},
	}
}

type CoKind uint8

const (
	CoLiteral CoKind = iota + 1
	CoSome
	CoOk
	CoExistential
	CoFnItem
	CoClosureErase
	CoNever
	// CoBorrow lends a Copy value to a `&T` parameter through a temporary.
	CoBorrow
	// CoCFnPtr turns an export(C) function item into a C function pointer.
	CoCFnPtr
	// CoLendExistential views a `&T` as a `&Interface` without copying or owning the value.
	CoLendExistential
)

type CoStep struct {
	Kind CoKind
	To   TypeID
}

// Coercion lists the steps applied to a synthesized value, innermost first.
type Coercion struct {
	From  TypeID
	Steps []CoStep
}

type FactKind uint8

const (
	FactIs FactKind = iota
	FactIsNot
	// FactPureField remembers a `pure?` field's bound purity at a place.
	FactPureField
)

type NarrowFact struct {
	Place   PlaceID
	Kind    FactKind
	Variant EntityID
	Mods    Effects
	Target  EntityID
	Param   EntityID
}

type EdgeKind uint8

const (
	EdgeCall EdgeKind = iota + 1
	EdgeCallback
	EdgeWitness
	EdgeIO
	EdgeMutateCaller
	EdgeUnsafe
	EdgeAlloc
	EdgeDrop
	EdgeContract
	EdgePanic
	EdgeParamCall
	// EdgeWitnessCall is a `pure?` requirement deferred to the caller's
	// resolved witness.
	EdgeWitnessCall
	// EdgeSuspend marks an `await` point: FactSuspend's only direct source.
	EdgeSuspend
	// EdgeIfaceArg carries an argument's known witnesses (Target) to the
	// parameter (Param) it binds; root-bridge reachability only.
	EdgeIfaceArg
	// EdgeKnownCall is a call to a statically known fn item (Target); root-bridge only.
	EdgeKnownCall
	// EdgeFieldBind binds a call-site argument's known fn (Target) to the
	// callee's field-param entity (Param); root-bridge reachability only.
	EdgeFieldBind
	// EdgeFnBoundArg carries a Fn-bound argument's known fn/closure (Target,
	// or a forwarded param) to the parameter (Param) it binds; root-bridge only.
	EdgeFnBoundArg
	// EdgeAsm is an asm block (Param 0) or a function one of its `sym` operands names (Param); op summary only.
	EdgeAsm
)

// fieldFnKey names one record field of a value held by Ent (a local or a parameter).
type fieldFnKey struct {
	Ent   EntityID
	Field EntityID
}

type EffectEdge struct {
	Kind   EdgeKind
	Target EntityID
	Param  EntityID
	Mods   Effects
	Type   TypeID
	From   TypeID
	File   FileID
	Node   syntax.NodeID
	Why    uint8
	// Captured marks a read of a capture, which is raw when the closure keeps its environment inline.
	Captured bool
	// Scoped edges sit inside an `allocator` block, which supplies the
	// current allocator for their dynamic extent.
	Scoped bool
	// Offloaded marks an EdgeCallback whose target runs on a bounded blocking provider's worker, not the
	// caller's own frame; op_summary_infer merges the target's facts as Offloaded regardless of their own cause.
	Offloaded bool
}

type BodyRef struct {
	Fn   EntityID
	File FileID
	Node syntax.NodeID
}

type Instance struct {
	Args []TypeID
	File FileID
	Node syntax.NodeID
	In   EntityID
}

func (f *FileInfo) span(n syntax.NodeID) token.Span { return f.Tree.Span(n) }
