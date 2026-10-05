package sem

import (
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

	target          Target
	noDefault       bool
	recoverableOnly bool
	noPlatform      bool
	blockingProfile string
	denyPkgs        map[string]bool
	layerCeiling    Layer
	hasCeiling      bool
	artifactName    string
	arch            string
	universe        ScopeID
	prelude         ScopeID
	pathIndex       map[string]PackageID
	fileIndex       map[*syntax.Tree]FileID
	placeIndex      map[string]PlaceID
	seen            map[diagKey]struct{}
	derives         []deriveRequest
	generated       []generatedFile
	mod             *Module
	sources         *token.SourceStore
	langItems       map[string]EntityID

	langItemMissing map[string]bool
	dropMemo        map[TypeID]EffectSummary
	opDropMemo      map[TypeID]OpSummary
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
	declScopes      map[EntityID]ScopeID
	aliasState      map[EntityID]uint8
	aliasParams     map[EntityID][]EntityID
	primEntity      map[TypeID]EntityID
	variantNames    map[PackageID]map[string][]EntityID
	conformMemo     map[conformKey]conformResult
	// localWitness is an immutable local's witnesses when its let
	// initializer coerces it to an interface; check_root_bridge.go reads it.
	localWitness map[EntityID][]EntityID
	// knownFn is a local/param's known fn item target(s) traced through
	// `let` aliasing; check_root_bridge.go uses it for a captured-variable call.
	knownFn map[EntityID][]EntityID
	// knownFieldFn is a record field's known fn item target(s), keyed by
	// the local its literal was bound to; check_root_bridge.go uses it for a stored field.
	knownFieldFn map[fieldFnKey][]EntityID
	// fieldParamEnts caches the synthetic EntFieldParam minted for one
	// (parameter, field) pair; see EntFieldParam.
	fieldParamEnts map[fieldFnKey]EntityID
	// fieldWitnessOf is a record field's source local/param, keyed by the
	// local its literal was bound to; a reassigned field is deleted, not stale.
	fieldWitnessOf          map[fieldFnKey]EntityID
	identPaths              map[*syntax.Tree]map[syntax.NodeID]syntax.NodeID
	pendingWitnessCallbacks []pendingWitnessCallback
	// lifetimeEscapeSeen dedupes an escaping (carrier, arg) pair.
	lifetimeEscapeSeen map[lifetimeEscapeKey]bool
	// provides maps a provider type's EntType entity to the context keys its
	// fields supply, declared by `@provides(...)`.
	provides map[EntityID][]ProvideBinding
	// ctxValueRefs are fn-value refs pending a post-fixpoint CtxReal check.
	ctxValueRefs    []ctxValueRef
	selfWholeAssign map[EntityID]bool
	borrowRecvCalls []borrowRecvCall
}
