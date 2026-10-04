package sem

import (
	"cmp"

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
	mod             *Module
	sources         *token.SourceStore
	langItems       map[string]EntityID

	langItemMissing         map[string]bool
	dropMemo                map[TypeID]EffectSummary
	opDropMemo              map[TypeID]OpSummary
	threadOpen              map[threadKey]int
	threadLow               int
	pkgEntity               []EntityID
	fileScopes              []ScopeID
	implicitMain            map[PackageID]EntityID
	importEdges             [][]PackageID
	stdImportGraph          map[PackageID][]PackageID
	pendingImports          []pendingImport
	order                   []PackageID
	bodiesChecked           bool
	declScopes              map[EntityID]ScopeID
	aliasState              map[EntityID]uint8
	aliasParams             map[EntityID][]EntityID
	primEntity              map[TypeID]EntityID
	variantNames            map[PackageID]map[string][]EntityID
	conformMemo             map[conformKey]conformResult
	identPaths              map[*syntax.Tree]map[syntax.NodeID]syntax.NodeID
	pendingWitnessCallbacks []pendingWitnessCallback
	// lifetimeEscapeSeen dedupes an escaping (carrier, arg) pair.
	lifetimeEscapeSeen map[lifetimeEscapeKey]bool
	// provides maps a provider type's EntType entity to the context keys its
	// fields supply, declared by `@provides(...)`.
	provides map[EntityID][]ProvideBinding
	// ctxValueRefs are fn-value refs pending a post-fixpoint CtxReal check.
	ctxValueRefs []ctxValueRef
}

func check(mod *Module) *Result {
	r := newResult(mod)
	r.prepareProgram()
	r.collect()
	r.resolveImports()
	r.declareTypes()
	r.declareSignatures()
	r.declareConsts()
	r.declareStatics()
	r.declareContextKeys()
	r.checkContextProviders()
	r.checkImpls()
	r.closeFieldParamForwarding()
	r.checkBodies()
	r.checkAccelKeys()
	r.resolvePendingWitnessCallbacks()
	r.checkEffects()
	r.checkContextRequirements()
	r.checkInterruptHandlers()
	r.checkAsyncDirectBlocking()
	r.checkNomono()
	r.reportUnused()
	r.emit()
	return r
}

func newResult(mod *Module) *Result {
	if mod.Sources == nil {
		mod.Sources = &token.SourceStore{}
	}
	for _, p := range mod.Packages {
		for _, t := range p.Files {
			if t.File.ID == 0 {
				mod.Sources.Add(t.File)
			}
		}
	}
	r := &Result{
		sources:         mod.Sources,
		mod:             mod,
		Types:           newTypeTable(cmp.Or(mod.PtrBits, 64)),
		Entities:        []Entity{{}},
		Scopes:          []Scope{{}},
		Overloads:       []OverloadSet{{}},
		Places:          []Place{{}},
		Instances:       map[EntityID][]Instance{},
		target:          mod.Target,
		noDefault:       mod.NoDefaultAllocator,
		recoverableOnly: mod.RecoverableOnly,
		noPlatform:      mod.NoPlatform,
		blockingProfile: mod.BlockingProfile,
		denyPkgs:        denySet(mod.Deny),
		layerCeiling:    LayerShortName[mod.LayerCeiling],
		hasCeiling:      mod.LayerCeiling != "",
		artifactName:    mod.ArtifactName,
		arch:            cmp.Or(mod.Arch, "amd64"),
		pathIndex:       map[string]PackageID{},
		fileIndex:       map[*syntax.Tree]FileID{},
		placeIndex:      map[string]PlaceID{},
		seen:            map[diagKey]struct{}{},
		langItems:       map[string]EntityID{},

		langItemMissing: map[string]bool{},
		implicitMain:    map[PackageID]EntityID{},
		declScopes:      map[EntityID]ScopeID{},
		aliasState:      map[EntityID]uint8{},
		aliasParams:     map[EntityID][]EntityID{},
		primEntity:      map[TypeID]EntityID{},
		variantNames:    map[PackageID]map[string][]EntityID{},
		conformMemo:     map[conformKey]conformResult{},
		dropMemo:        map[TypeID]EffectSummary{},
		opDropMemo:      map[TypeID]OpSummary{},
		identPaths:      map[*syntax.Tree]map[syntax.NodeID]syntax.NodeID{},
		provides:        map[EntityID][]ProvideBinding{},
	}
	r.Files = []FileInfo{{}}
	r.Packages = []PackageInfo{{Path: "", Module: ""}}
	r.universe = r.newScope(ScopeUniverse, 0, 0, 0)
	r.prelude = r.newScope(ScopePrelude, r.universe, 0, 0)
	r.declareUniverse()
	r.addPackages(mod)
	return r
}
