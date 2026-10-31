package llgen

import (
	"strings"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type emitter struct {
	p            *mir.Program
	entryAllocas []string
	cancelFlag   string
	hoisting     bool
	freestanding bool
	cabi         cIntABI
	entrySymbol  string
	reachable    *mir.Reachability
	r            *sem.Result
	out          strings.Builder
	strs         map[string]string
	strList      []string
	descs        map[sem.EntityID]string
	vdescs       map[sem.EntityID]string
	primDescs    map[string]string
	tables       strings.Builder
	foreign      map[string]string
	helpers      strings.Builder
	cio          map[string]bool
	ntypes       int
	tmp          int
	stdDrops     map[sem.EntityID]bool
	shim         *cshim
	modAsm       strings.Builder
	symPrefix    string
	scalar       map[midir.LocalID]scalarInfo
	raw          map[midir.LocalID]bool
	fnItems      map[string]fnItem
	stdBinds     []stdBind
	stdBindOf    map[string]string
	constVals    map[string]constVal
	constShape   map[string]constVal
	aggLabels    map[string]bool
	constIndex   map[string]int
	constOrder   []string
	constKey     map[midir.LocalID]string
	constOwned   map[midir.LocalID]bool
	aggs         map[sem.TypeID]*aggInfo
	aggOrder     []sem.TypeID
	aggDone      map[sem.TypeID]bool
	protoDone    map[protoKey]bool
	direct       map[midir.LocalID]*aggInfo
	boxTmp       map[midir.LocalID]bool
	crossSrc     map[midir.LocalID]midir.LocalID
	countedX     map[midir.LocalID]countedCross
	refDone      map[string]bool
	nameAlias    map[midir.LocalID]bool
	lentCells    map[midir.LocalID]bool
	sigs         map[*midir.Func]*aggSig
	sig          *aggSig
	ssigs        map[*midir.Func]*scalarSig
	ssig         *scalarSig
	wrapNeed     map[wrapperKey]bool
	wrapDone     map[wrapperKey]bool
	catch        bool
	abiSym       string
	unwind       *unwindFrame
	outer        *outerDrops
	at           siteAt
	std          stdUse
	intrinsics   map[string]string

	immortalStatics map[sem.EntityID]bool
	// boxes is nil during final emission; recording must not change the generated program.
	boxes *boxRecord
}
