package sem

import (
	"sync"

	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

type PackageInfo struct {
	Path      string
	Module    string
	Parent    PackageID
	Scope     ScopeID
	SCC       int
	Entry     FileID
	HasScript bool
	Std       bool
	Files     []FileID
	tests     map[string]EntityID
}

type Result struct {
	Types     *TypeTable
	Entities  []Entity
	Scopes    []Scope
	Files     []FileInfo
	Packages  []PackageInfo
	Bodies    []BodyRef
	Overloads []OverloadSet
	Places    []Place
	Instances map[EntityID][]Instance

	TypeDecls  []TypeDeclInfo
	Fns        []FnInfo
	Ifaces     []InterfaceInfo
	Consts     []ConstInfo
	Statics    []StaticInfo
	Locals     []LocalInfo
	Closures   []ClosureInfo
	Fields     []FieldInfo
	Variants   []VariantInfo
	TypeParams []TypeParamInfo
	// Comptime lists the blocks the driver must evaluate before lowering.
	Comptime []ComptimeSite

	parseFailed     bool
	replay          *replayedCheck
	target          Target
	noDefault       bool
	recoverableOnly bool
	noPlatform      bool
	blockingProfile string
	denyPkgs        map[string]bool
	layerCeiling    Layer
	hasCeiling      bool
	artifactName    string
	failedImports   map[FileID]map[string]bool
	arch            string
	universe        ScopeID
	prelude         ScopeID
	pathIndex       map[string]PackageID
	fileIndex       map[*syntax.Tree]FileID
	placeIndex      map[string]PlaceID
	seen            map[diagKey]struct{}
	derives         []deriveRequest
	generated       []generatedFile
	// genFailed marks a DeriveGenerator failure (E1430, E1432); CheckWith
	// skips its rerun then, rather than burying the cause behind missing-witness errors.
	genFailed bool
	mod       *Module
	sources   *token.SourceStore
	langItems map[string]EntityID

	langItemMissing map[string]bool
	dropMemo        map[TypeID]EffectSummary
	opDropMemo      map[TypeID]OpSummary
	fatalFamily     map[string]bool
	threadOpen      map[threadKey]int
	threadLow       int
	pkgEntity       []EntityID
	fileScopes      []ScopeID
	implicitMain    map[PackageID]EntityID
	importEdges     [][]PackageID
	stdImportGraph  map[PackageID][]PackageID
	pendingImports  []pendingImport
	order           []PackageID
	bodiesChecked   bool
	cancelled       bool
	declScopes      map[EntityID]ScopeID
	aliasState      map[EntityID]uint8
	aliasParams     map[EntityID][]EntityID
	primEntity      map[TypeID]EntityID
	variantNames    map[PackageID]map[string][]EntityID
	conformMemo     map[conformKey]conformResult
	// localWitness is an immutable local's witnesses when its let
	// initializer coerces it to an interface; check_root_bridge.go reads it.
	localWitness map[EntityID][]EntityID
	// ifaceResult is the witnesses a fn's interface result can come from; absent when a return is untracked.
	ifaceResult map[EntityID][]EntityID
	// ifaceRecvFn is the fn whose call result is an interface call's receiver.
	ifaceRecvFn map[literalEntKey]EntityID
	// knownFn is a local/param's known fn item target(s) traced through
	// `let` aliasing; check_root_bridge.go uses it for a captured-variable call.
	knownFn map[EntityID][]EntityID
	// knownFieldFn is a record field's known fn item target(s), keyed by
	// the local its literal was bound to; check_root_bridge.go uses it for a stored field.
	knownFieldFn map[fieldFnKey][]EntityID
	// fieldParamEnts caches the synthetic EntFieldParam minted for one
	// (parameter, field) pair; see EntFieldParam.
	fieldParamEnts map[fieldFnKey]EntityID
	// fieldWitnessOf is a never-reassignable field's source local/param (owner
	// not `mut`), aliased directly so per-call-site narrowing still sees through it.
	fieldWitnessOf map[fieldFnKey]EntityID
	// fieldWitnessPoisoned sticks once a field write isn't statically known, so
	// a later known write can't resurrect it; read indirectly via absence.
	fieldWitnessPoisoned map[fieldFnKey]bool
	// methodFieldCalls stage a `mut self` method call site for
	// closeMethodFieldWrites, run once every body is checked.
	methodFieldCalls []methodFieldCallSite
	// fieldBindOwners is a field-param entity's EdgeFieldBind/EdgeIfaceArg
	// owners, so poisonFieldKeyGlobal can retract a now-stale edge.
	fieldBindOwners     map[EntityID][]EntityID
	fieldPoisonForwards []fieldPoisonForward
	lateFieldEdges      []lateFieldEdge
	lateBindSites       []lateBindSite
	retractedBinds      map[bindSite]bool
	fnLocalField        map[EntityID]fieldFnKey
	fnLocalFieldRead    map[EntityID]syntax.NodeID
	refAlias            map[EntityID]EntityID
	fnCopyAlias         map[EntityID]EntityID
	// fieldPoisonSite is best-effort: not every unresolved write is tracked.
	fieldPoisonSite map[fieldFnKey]fieldPoisonSiteInfo
	// literalFieldEnts caches literalFieldEnt's synthetic entity per literal node.
	literalFieldEnts        map[literalEntKey]EntityID
	fnUnions                []fnUnion
	fnFieldUnions           []fnFieldUnion
	noallocErase            map[literalEntKey]bool
	flow                    flowState
	fieldFnSites            map[fieldSiteKey]fieldPoisonSiteInfo
	edgeFnSites             map[edgeSiteKey]fieldPoisonSiteInfo
	fieldKeyByEnt           map[EntityID]fieldFnKey
	nestedCalled            map[EntityID][]nestedFieldKey
	fnResults               map[EntityID]*fnResult
	fnResultWrites          []fnResultWrite
	resultEnts              map[literalEntKey]EntityID
	containerEnts           map[literalEntKey]EntityID
	containerLits           map[EntityID]bool
	containerFields         [4]EntityID
	fnResultEnts            map[EntityID]EntityID
	identPaths              map[*syntax.Tree]map[syntax.NodeID]syntax.NodeID
	pendingWitnessCallbacks []pendingWitnessCallback
	// lifetimeEscapeSeen dedupes an escaping (carrier, arg) pair.
	lifetimeEscapeSeen map[lifetimeEscapeKey]bool
	taskEscapeSeen     map[taskEscapeKey]bool
	taskFutureUse      map[taskEscapeKey]uint8
	// provides maps a provider type's EntType entity to the context keys its
	// fields supply, declared by `@provides(...)`.
	provides map[EntityID][]ProvideBinding
	// ctxValueRefs are fn-value refs pending a post-fixpoint CtxReal check.
	ctxValueRefs []ctxValueRef
	// ctxImplicit: closures that capture a provider local only through a call's hidden argument.
	ctxImplicit map[EntityID][]EntityID
	// ctxDeferred: exclusive-access judgements on a provider, run once ctxImplicit is complete.
	ctxDeferred     []func()
	selfWholeAssign map[EntityID]bool
	// refCell* decide `&mut` cell lowering (resolve_ref_cells.go); the fixpoint runs once every body is checked.
	refCellRecvs              []refCellRecv
	refCellSeeds              []EntityID
	refCellEdges              []refCellEdge
	refCellSites, refCellArgs []refCellSite
	asyncValues               []refCellSite
	refCellValues             []refCellValue
	refCellParams             map[EntityID]bool
	refCellHard               map[EntityID]bool
	refCellEdgeList           []refCellEdge
	selfLentArgs              []selfLentArg
	selfRecvCalls             []selfRecvCall
	witnessPairs              []witnessPair
	witnessSeen               map[witnessPair]bool
	conformSealed             int
	instances                 map[EntityID]InstanceInfo
	instanceKeys              map[string]EntityID
	closureOrigins            map[EntityID]EntityID
	ordinalMu                 sync.Mutex
	varNames                  []string
	closureOrdinals           map[EntityID]int
	lifetimeStandIns          []EntityID
	localLifetimes            map[localLifetimeKey]EntityID
	localLifetimeFn           EntityID
	deferredClaims            bool
	boxedParams               map[EntityID]bool
	boxedForwards             []boxedForward
	lendSites                 map[lendSite]lendSite
	boxedLends                map[lendSite]bool
	recoverable               recoverableProfile
	pendingLends              []lendSite
	allocBlocks               []allocBlock
	skipBody, lazyPkg         []bool
}
