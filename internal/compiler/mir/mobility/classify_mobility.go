package mobility

import (
	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Classify answers whether the frame may move between workers: all
// Send is migratable, and the first state that is not pins the Future.
func Classify(f *midir.Func, r *sem.Result) {
	if e := r.Entity(f.Ent); e.Kind != sem.EntFn {
		return
	}
	crossing := frameCrossing(f)
	info := r.Fn(f.Ent)
	info.Mobility, info.BoundBy, info.BoundRead = sem.MobilityMigratable, "", sem.ReadSite{}
	for i, l := range f.Locals {
		if !crossing[midir.LocalID(i)] || r.IsSendInFrame(l.Type, f.Ent) {
			continue
		}
		info.Mobility, info.BoundBy = sem.MobilityExecutorBound, r.TypeString(l.Type)
		return
	}
}

// A migratable Future changes hands three times: the spawning lane fills the
// initial frame, a worker resumes each suspension state, the awaiter takes T.
func frameCrossing(f *midir.Func) map[midir.LocalID]bool {
	at := map[midir.LocalID]bool{}
	for _, p := range f.Params {
		at[p] = true
	}
	for _, blk := range f.Blocks {
		if blk.Term.Op == midir.TermReturn {
			for _, a := range blk.Term.Args {
				at[a] = true
			}
		}
	}
	for _, live := range mirflow.LiveAtSuspends(f) {
		live.Each(func(l midir.LocalID) { at[l] = true })
	}
	return at
}
