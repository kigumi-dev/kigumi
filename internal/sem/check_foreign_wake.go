package sem

// A wake is only safe from a running task, and a C caller's thread is not one.
func (r *Result) checkForeignWake() {
	cancel := r.taskCancel()
	if cancel == 0 {
		return
	}
	var entries []EntityID
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind == EntFn && e.Flags&EfPoison == 0 && r.Fn(EntityID(id)).Export == "C" {
			entries = append(entries, EntityID(id))
		}
	}
	if len(entries) == 0 {
		return
	}
	rr := r.inferRunnerReach(r.effectNodes(), cancel, "wakes a task here", true)
	for _, id := range entries {
		if rr.dist[id] > 0 {
			r.reportRunnerReach(id, rr, cForeignWake, r.Entities[id].Name)
		}
	}
}

func (r *Result) taskCancel() EntityID {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind != EntFn || e.Name != "cancel" || r.Packages[e.Pkg].Path != "std/task" {
			continue
		}
		if owner := r.Fn(EntityID(id)).Owner; owner != 0 && r.Entities[owner].Name == "Task" {
			return EntityID(id)
		}
	}
	return 0
}
