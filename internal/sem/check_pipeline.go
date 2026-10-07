package sem

import (
	"cmp"

	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

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
	r.bindLateFieldCalls()
	r.resolveRefCells()
	r.checkAccelKeys()
	r.resolvePendingWitnessCallbacks()
	// Must precede checkEffects: it appends hidden provider captures to
	// ClosureInfo.Captures, which computeDrop/computeDropOp read.
	r.checkContextRequirements()
	r.checkEffects()
	r.checkInterruptHandlers()
	r.checkAsyncDirectBlocking()
	r.checkThreadPool()
	r.closeMethodFieldWrites()
	r.closeLateFieldEdges()
	r.retractPoisonedBindings()
	r.checkRootBridges()
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

		selfWholeAssign:      map[EntityID]bool{},
		langItemMissing:      map[string]bool{},
		implicitMain:         map[PackageID]EntityID{},
		declScopes:           map[EntityID]ScopeID{},
		aliasState:           map[EntityID]uint8{},
		aliasParams:          map[EntityID][]EntityID{},
		primEntity:           map[TypeID]EntityID{},
		variantNames:         map[PackageID]map[string][]EntityID{},
		conformMemo:          map[conformKey]conformResult{},
		witnessSeen:          map[witnessPair]bool{},
		localWitness:         map[EntityID][]EntityID{},
		knownFn:              map[EntityID][]EntityID{},
		knownFieldFn:         map[fieldFnKey][]EntityID{},
		fieldParamEnts:       map[fieldFnKey]EntityID{},
		fieldWitnessOf:       map[fieldFnKey]EntityID{},
		fieldWitnessPoisoned: map[fieldFnKey]bool{},
		dropMemo:             map[TypeID]EffectSummary{},
		opDropMemo:           map[TypeID]OpSummary{},
		identPaths:           map[*syntax.Tree]map[syntax.NodeID]syntax.NodeID{},
		provides:             map[EntityID][]ProvideBinding{},
	}
	r.Files = []FileInfo{{}}
	r.Packages = []PackageInfo{{Path: "", Module: ""}}
	r.universe = r.newScope(ScopeUniverse, 0, 0, 0)
	r.prelude = r.newScope(ScopePrelude, r.universe, 0, 0)
	r.declareUniverse()
	r.addPackages(mod)
	return r
}
