package sem

import "kigumi/internal/syntax"

// threadPoolKey names the one std function that starts worker threads;
// mir.ThreadPoolKey is the same string on the backend side.
const threadPoolKey = "task.threadPoolExecutor"

// Rejects every mention of the pool constructor, not just a call: mir.UsesThreads
// links on a mention too, so a call-only check would miss a function value.
func (r *Result) checkThreadPool() {
	code, ok := r.threadPoolBlockedCode()
	if !ok {
		return
	}
	pool := r.entityByStdKey(threadPoolKey)
	if pool == 0 {
		return
	}
	for f := 1; f < len(r.Files); f++ {
		for n, use := range r.Files[f].Uses {
			if use == pool {
				r.errAt(FileID(f), syntax.NodeID(n), code, r.Entities[pool].Name, r.blockingProfile)
			}
		}
	}
}

// ThreadPoolAllowed reports whether a thread-pool executor may drive this program, so handed-off work may run on another thread.
func (r *Result) ThreadPoolAllowed() bool {
	_, blocked := r.threadPoolBlockedCode()
	return !blocked
}

func (r *Result) threadPoolBlockedCode() (Code, bool) {
	switch {
	case r.target == Freestanding || r.noPlatform:
		return cThreadPoolTarget, true
	case r.blockingProfile != hostedBlockingProfile:
		return cThreadPoolProfile, true
	}
	return Code{}, false
}

func (r *Result) entityByStdKey(key string) EntityID {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind == EntFn && r.Packages[e.Pkg].Std && r.stdKey(EntityID(id)) == key {
			return EntityID(id)
		}
	}
	return 0
}
