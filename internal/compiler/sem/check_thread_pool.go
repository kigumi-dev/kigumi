package sem

import (
	"strings"

	"kigumi/internal/compiler/syntax"
)

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

// entityByStdKey resolves a stdKey ("task.threadPoolExecutor") through the
// package scope, and remembers the answer: the call check asks for the pool
// once per call site, so scanning every entity (and building its key) per call
// made a trivial module's check quadratic in the module size.
func (r *Result) entityByStdKey(key string) EntityID {
	if id, ok := r.stdKeyMemo[key]; ok {
		return id
	}
	id := r.lookupStdKey(key)
	if r.stdKeyMemo == nil {
		r.stdKeyMemo = map[string]EntityID{}
	}
	r.stdKeyMemo[key] = id
	return id
}

func (r *Result) lookupStdKey(key string) EntityID {
	dot := strings.IndexByte(key, '.')
	last := strings.LastIndexByte(key, '.')
	if dot <= 0 || last < dot {
		return 0
	}
	pkg, ok := r.pathIndex["std/"+key[:dot]]
	if !ok {
		return 0
	}
	b, ok := r.Scopes[r.Packages[pkg].Scope].Names[key[last+1:]]
	if !ok {
		return 0
	}
	if b.Ent != 0 && r.Entities[b.Ent].Kind == EntFn && r.stdKey(b.Ent) == key {
		return b.Ent
	}
	for _, m := range r.Overloads[b.Set].Members {
		if r.Entities[m].Kind == EntFn && r.stdKey(m) == key {
			return m
		}
	}
	return 0
}
