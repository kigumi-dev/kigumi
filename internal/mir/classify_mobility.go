package mir

import "kigumi/internal/sem"

// classifyMobility answers whether the frame may move between workers: all
// Send is migratable, and the first state that is not pins the Future.
func classifyMobility(f *Func, r *sem.Result) {
	if e := r.Entity(f.Ent); e.Kind != sem.EntFn {
		return
	}
	crossing := frameCrossing(f)
	info := r.Fn(f.Ent)
	info.Mobility, info.BoundBy = sem.MobilityMigratable, ""
	for i, l := range f.Locals {
		if !crossing[LocalID(i)] || r.IsSend(l.Type) {
			continue
		}
		info.Mobility, info.BoundBy = sem.MobilityExecutorBound, r.TypeString(l.Type)
		return
	}
}

// A migratable Future changes hands three times: the spawning lane fills the
// initial frame, a worker resumes each suspension state, the awaiter takes T.
func frameCrossing(f *Func) map[LocalID]bool {
	at := map[LocalID]bool{}
	for _, p := range f.Params {
		at[p] = true
	}
	for _, blk := range f.Blocks {
		if blk.Term.Op == TermReturn {
			for _, a := range blk.Term.Args {
				at[a] = true
			}
		}
	}
	for _, live := range liveAtSuspends(f) {
		live.each(func(l LocalID) { at[l] = true })
	}
	return at
}
