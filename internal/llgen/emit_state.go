package llgen

import (
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
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
	scalar       map[mir.LocalID]scalarInfo
	raw          map[mir.LocalID]bool
	fnItems      map[string]fnItem
	stdBinds     []stdBind
	stdBindOf    map[string]string
	constVals    map[string]constVal
	constShape   map[string]constVal
	aggLabels    map[string]bool
	constIndex   map[string]int
	constOrder   []string
	constKey     map[mir.LocalID]string
	constOwned   map[mir.LocalID]bool
	aggs         map[sem.TypeID]*aggInfo
	aggOrder     []sem.TypeID
	aggDone      map[sem.TypeID]bool
	protoDone    map[protoKey]bool
	direct       map[mir.LocalID]*aggInfo
	boxTmp       map[mir.LocalID]bool
	crossSrc     map[mir.LocalID]mir.LocalID
	countedX     map[mir.LocalID]countedCross
	refDone      map[string]bool
	nameAlias    map[mir.LocalID]bool
	lentCells    map[mir.LocalID]bool
	sigs         map[*mir.Func]*aggSig
	sig          *aggSig
	ssigs        map[*mir.Func]*scalarSig
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
	// boxes records the scalar boxes ScalarArgBoxes' lowering makes; nil when emitting.
	boxes *boxRecord
}
