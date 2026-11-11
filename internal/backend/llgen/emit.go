// Package llgen emits LLVM IR text from MIR.
package llgen

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/target"
)

// prepareEmitter is the emitter EmitProgram lowers with, its scalar ABI planned.
func prepareEmitter(p *mir.Program, opts EmitOptions) *emitter {
	e := &emitter{p: p, r: p.R, strs: map[string]string{}, descs: map[sem.EntityID]string{}, vdescs: map[sem.EntityID]string{}, foreign: map[string]string{}, stdDrops: map[sem.EntityID]bool{}, cio: map[string]bool{}, fnItems: map[string]fnItem{}, stdBindOf: map[string]string{}, constVals: map[string]constVal{}, constShape: map[string]constVal{}, immortalStatics: map[sem.EntityID]bool{}, constIndex: map[string]int{}, aggs: map[sem.TypeID]*aggInfo{}, aggDone: map[sem.TypeID]bool{}, refDone: map[string]bool{}, sigs: map[*midir.Func]*aggSig{}, wrapNeed: map[wrapperKey]bool{}, wrapDone: map[wrapperKey]bool{}, freestanding: opts.Freestanding, entrySymbol: opts.EntrySymbol, symPrefix: opts.SymbolPrefix, reachable: opts.Reachable, shim: newCshim()}
	e.catch = e.wantsCatch(opts)
	e.layout = opts.Layout
	if e.layout.Arch == "" {
		e.layout = target.Default()
	}
	if opts.AbiCheck {
		e.abiSym = e.cstr(AbiPlaceholder)
	}
	e.planScalarABI()
	return e
}

func (e *emitter) keeps(f *midir.Func) bool { return e.reachable == nil || e.reachable.Keep(f.Ent) }

func (e *emitter) naked(f *midir.Func) bool {
	return e.r.Entity(f.Ent).Kind == sem.EntFn && e.r.Fn(f.Ent).Naked
}

func EmitProgram(p *mir.Program, opts EmitOptions) Output {
	e := prepareEmitter(p, opts)
	var body strings.Builder
	for _, f := range p.Funcs {
		if !e.keeps(f) {
			continue
		}
		if e.naked(f) {
			e.nakedFn(f)
			continue
		}
		if f.Async {
			body.WriteString(e.asyncFunction(f))
			e.helpers.WriteString(e.asyncDesc(f))
			continue
		}
		body.WriteString(e.function(f))
		if e.r.Entity(f.Ent).Kind == sem.EntFn && e.r.Fn(f.Ent).Export != "" {
			switch {
			case e.r.Fn(f.Ent).Export == "interrupt":
				body.WriteString(e.exportInterruptWrapper(f))
			case e.byValue(e.r.Fn(f.Ent).Sig):
				body.WriteString(e.exportByval(f))
			default:
				body.WriteString(e.exportWrapper(f))
			}
		}
	}
	// Both add foreign declarations before those are written, and a static's
	// initializer adds literal pool and immortal slots that data() emits.
	globalDecls, globalInit := e.staticGlobals()
	main := e.mainSym()
	body.WriteString(e.emitWrappers())
	data := e.data()
	// data() can name a wrapper nothing else asked for.
	body.WriteString(e.emitWrappers())
	int128 := e.usesInt128()
	e.out.WriteString(header)
	e.out.WriteString(e.aggTypes())
	e.out.WriteString(memIntrinsics)
	e.out.WriteString(e.intrinsicDecls())
	e.out.WriteString(e.modAsm.String())
	e.out.WriteString(externs)
	for _, name := range slices.Sorted(maps.Keys(e.foreign)) {
		e.out.WriteString(e.foreign[name] + "\n")
	}
	e.out.WriteString(globalDecls)
	e.out.WriteString(data)
	e.out.WriteString(body.String())
	e.out.WriteString(globalInit)
	e.out.WriteString(e.helpers.String())
	e.out.WriteString(e.initImmortals())
	e.out.WriteString(e.mainWrapper(main))
	var caps []string
	if opts.Reachable != nil {
		caps = opdef.CapabilityMacros(opts.Reachable.Ops)
	}
	ir := e.out.String()
	if e.freestanding {
		ir = markNoUnwind(ir)
	}
	return Output{IR: ir, CShim: e.shim.text(), Capabilities: caps, Unwind: e.catch, Int128: int128, StdOps: e.std.macros(), Foreign: slices.Sorted(maps.Keys(e.foreign))}
}

func (e *emitter) newTmp() string {
	e.tmp++
	return fmt.Sprintf("%%v%d", e.tmp)
}

// classifyBody fixes how f's locals are held; function and asyncFunction lower on it.
func (e *emitter) classifyBody(f *midir.Func) {
	e.direct = e.classifyDirect(f)
	e.boxTmp = boxTemps(f)
	e.crossSrc = e.crossSources(f)
	e.countedX = e.countedCrosses(f)
	e.scalar = e.classifyScalars(f)
	e.nameAlias = nameAliases(f)
	e.lentCells = nil
	e.scalarAliasBoxes(f)
	e.constOwned = map[midir.LocalID]bool{}
	e.constKey = e.constLocals(f)
}

func (e *emitter) function(f *midir.Func) string {
	var sb strings.Builder
	e.tmp = 0
	e.sig, e.ssig = e.directSig(f), e.ssigs[f]
	e.classifyBody(f)
	e.unwind = e.unwindFor(f)
	e.outer = e.outerDropsFor(f)
	sb.WriteString(e.sigHeader(f, e.sig))
	if e.r.Entity(f.Ent).Kind == sem.EntFn {
		if name := e.r.FnSectionOf(f.Ent); name != "" {
			fmt.Fprintf(&sb, " section %q", name)
		}
	}
	sb.WriteString(" {\nentry:\n")
	e.unwindAllocas(&sb)
	for i := range f.Locals {
		e.frameLocal(&sb, midir.LocalID(i))
	}
	var body strings.Builder
	e.entryAllocas, e.hoisting = nil, true
	stores := e.p.CopyStores(f)
	for bid, blk := range f.Blocks {
		fmt.Fprintf(&body, "b%d:\n", bid)
		e.unwindBlock()
		for pc, in := range copyStoreInsts(&blk, bid, stores) {
			e.at = siteAt{midir.BlockID(bid), pc}
			e.unwindSite(&body, e.at.blk, pc)
			e.inst(&body, f, in)
		}
		e.term(&body, blk.Term)
	}
	e.outerFailBlocks(&body)
	e.hoisting = false
	for _, a := range e.entryAllocas {
		sb.WriteString(a)
	}
	// Parameters arrive owned or borrowed; either way, store as is.
	e.storeParams(&sb, f, e.sig)
	e.unwindEnter(&sb)
	sb.WriteString("  br label %b0\n")
	sb.WriteString(body.String())
	sb.WriteString("}\n\n")
	e.unwind, e.outer = nil, nil
	return sb.String()
}
