package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	resbound "kigumi/internal/compiler/mir/resource"
	"kigumi/internal/compiler/sem"
)

type Program struct {
	R     *sem.Result
	Funcs []*midir.Func
	ByEnt map[sem.EntityID]*midir.Func
	Entry *midir.Func
	// Comptime lists the comptime blocks, each lowered as a function of its
	// own that the driver evaluates and folds before anything runs.
	Comptime []*ComptimeFunc
	// Resources is every function's resource summary, set once ComputeResourceSummaries runs; nil before then.
	Resources map[sem.EntityID]*resbound.Summary
	unwind    unwindMemo
	executor  executorMemo
	Diags     []midir.FileDiag
	// Entries holds every Func.Entry; they are not in Funcs, so nothing keyed by entity sees them.
	Entries []*midir.Func
	// VerifyEach makes Apply verify before the passes and after each one, not only after the last.
	VerifyEach bool
	unlowered  []bool
	// Skipped lists the bodies lowering needed that sem left unchecked; the program is unusable when it is non-empty.
	Skipped []sem.EntityID
	// NativeBoxes reports scalar allocations made by native lowering; profiles require this hook.
	NativeBoxes func(*Program) NativeBoxSites
}

// Unlowered reports whether ent has a body the builder left out because nothing reaches it.
func (p *Program) Unlowered(ent sem.EntityID) bool {
	return int(ent) < len(p.unlowered) && p.unlowered[ent]
}

func (p *Program) AllFuncs() []*midir.Func {
	fs := append([]*midir.Func{}, p.Funcs...)
	for _, cf := range p.Comptime {
		fs = append(fs, cf.Func)
	}
	return append(fs, p.Entries...)
}

// SetUnlowered records, by entity, the bodies the builder left out.
func (p *Program) SetUnlowered(left []bool) { p.unlowered = left }
