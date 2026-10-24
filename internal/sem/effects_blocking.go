package sem

// directBlockingPrimitives are bodiless std fns that hold the calling agent until the syscall returns.
var directBlockingPrimitives = map[string]bool{
	"net.getaddrinfo": true,
	"net.connect":     true,
	"net.accept":      true,
	"net.read":        true,
	"net.write":       true,
	"net.poll":        true,
	"fs.fopen":        true,
	"fs.fclose":       true,
	"fs.fflush":       true,
	"fs.fread":        true,
	"fs.fwrite":       true,
	"fs.remove":       true,
	"fs.access":       true,
	"fs.mkdir":        true,
	"fs.opendir":      true,
	"fs.readdir":      true,
	"fs.closedir":     true,
	"os.read":         true,
	"os.write":        true,
	"os.poll":         true,
	"shell.read":      true,
	"shell.write":     true,
	"shell.poll":      true,
	"shell.waitpid":   true,
}

// nonBlockingStd are bodied std fns that make their descriptor non-blocking first.
var nonBlockingStd = map[string]bool{
	"net.Listener.pollAccept": true,
	"net.Conn.pollRead":       true,
	"net.Conn.pollWrite":      true,
}

// trustedOpSummaries are hand-verified fact sets for bodiless std fns.
var trustedOpSummaries = map[string]OpSummary{
	"task.BlockingPool.new": {Known: true, Facts: [numOpFacts]OpFactState{
		FactAlloc: {Maybe: true},
	}},
	"task.BlockingPool.run": {Known: true, Facts: [numOpFacts]OpFactState{
		FactAlloc: {Maybe: true},
		FactBlock: {Maybe: true, Offloaded: true},
	}},
}

// offloadingCallbacks are std fns whose fn argument runs on a provider worker when threads exist.
var offloadingCallbacks = map[string]bool{
	"task.BlockingPool.run": true,
}

// A strict profile rejects threadPoolExecutor, so BlockingPool.run runs its closure on the caller.
func (r *Result) workerThreads() bool { return r.blockingProfile == hostedBlockingProfile }

func (r *Result) isOffloadingCallback(fn EntityID) bool {
	return r.workerThreads() && offloadingCallbacks[r.stdKey(fn)]
}

// checkAsyncDirectBlocking flags a blocking op reachable from an async body; hosted warns, strict rejects.
func (r *Result) checkAsyncDirectBlocking() {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		// Without worker threads a std pool closure runs on the caller, whose own async fn is reported.
		if e.Kind != EntFn || e.Flags&EfPoison != 0 || r.Packages[e.Pkg].Std && !r.workerThreads() {
			continue
		}
		info := r.Fn(EntityID(id))
		if info.Body == 0 || info.Declared&EffAsync == 0 {
			continue
		}
		r.reportAsyncBlock(EntityID(id))
	}
}

// hostedBlockingProfile is the "" CLI value: a direct-blocking reach warns
// instead of rejecting.
const hostedBlockingProfile = ""

func (r *Result) reportAsyncBlock(id EntityID) {
	fb := r.opSummary(id).Facts[FactBlock]
	// Unknown is the bodiless default's blanket widening, not a verified
	// block; only a directBlockingPrimitives-sourced concrete cause reports.
	if !fb.Maybe || fb.Offloaded || fb.Cause.Unknown {
		return
	}
	e := &r.Entities[id]
	profile := r.blockingProfile
	code := cAsyncDirectBlockWarn
	var args []any
	if profile == hostedBlockingProfile {
		args = []any{e.Name}
	} else {
		code = cAsyncDirectBlockStrict
		args = []any{e.Name, profile}
	}
	d, ok := r.diagAt(e.File, e.Node, code, args...)
	if !ok {
		return
	}
	for _, h := range r.ExplainOpFact(id, FactBlock).Chain {
		d = d.WithNote(r.loc(h.File, r.Files[h.File].span(h.Node)), h.Text)
	}
	r.emitDiag(e.File, d)
}
