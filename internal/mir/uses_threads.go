package mir

import "kigumi/internal/sem"

// ThreadPoolKey is the std function whose reachability decides whether a
// build links the thread runtime and makes refcount updates atomic.
const ThreadPoolKey = "task.threadPoolExecutor"

func UsesThreads(p *Program) bool {
	for _, f := range p.allFuncs() {
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Ent != 0 && isThreadPool(p.R, in.Ent) {
					return true
				}
			}
		}
	}
	return false
}

func isThreadPool(r *sem.Result, ent sem.EntityID) bool {
	e := r.Entity(ent)
	if e.Kind != sem.EntFn || !r.Packages[e.Pkg].Std {
		return false
	}
	return StdKey(r, ent, true) == ThreadPoolKey
}
