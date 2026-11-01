package reach

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Body is a lowered function the walk scans as if Ent reached it.
type Body struct {
	Func *midir.Func
	Ent  sem.EntityID
}

// Lower lowers, through lower, whatever the roots and the scanned bodies reach.
func Lower(r *sem.Result, byEnt map[sem.EntityID]*midir.Func, roots []sem.EntityID, scanned []Body, lower func(sem.EntityID), lowerable func(sem.EntityID) bool) {
	w := newReachWalk(r, byEnt)
	w.lower, w.lowerable = lower, lowerable
	for _, id := range roots {
		if w.rs.add(id, Reason{}) {
			w.worklist = append(w.worklist, id)
		}
	}
	for _, b := range scanned {
		w.scan(b.Func, b.Ent, r.Entity(b.Ent).Pkg)
	}
	w.drain()
}
