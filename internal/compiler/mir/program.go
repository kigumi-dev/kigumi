package mir

import "kigumi/internal/compiler/sem"

type Program struct {
	R     *sem.Result
	Funcs []*Func
	ByEnt map[sem.EntityID]*Func
	Entry *Func
	// Comptime lists the comptime blocks, each lowered as a function of its
	// own that the driver evaluates and folds before anything runs.
	Comptime []*ComptimeFunc
	// Resources is every function's resource summary, set once ComputeResourceSummaries runs; nil before then.
	Resources map[sem.EntityID]*ResourceSummary
	unwind    unwindMemo
	sole      *soleIndex
	executor  executorMemo
	offloads  offloadMemo
	Diags     []FileDiag
	// Entries holds every Func.Entry; they are not in Funcs, so nothing keyed by entity sees them.
	Entries []*Func
	// VerifyEach makes Apply verify before the passes and after each one, not only after the last.
	VerifyEach bool
	lowering   *lowering
	unlowered  []bool
	// rootKeep, when set, limits the up-front lowering to the packages it accepts.
	rootKeep func(path string) bool
	// Skipped lists the bodies lowering needed that sem left unchecked; the program is unusable when it is non-empty.
	Skipped []sem.EntityID
	// NativeBoxes reports scalar allocations made by native lowering; profiles require this hook.
	NativeBoxes func(*Program) NativeBoxSites

	paramStores map[paramSlot]bool
}

// Unlowered reports whether ent has a body Build left out because nothing reaches it.
func (p *Program) Unlowered(ent sem.EntityID) bool {
	return int(ent) < len(p.unlowered) && p.unlowered[ent]
}
