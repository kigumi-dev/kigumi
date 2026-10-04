package sem

// directBlockingPrimitives lists bodiless std fns (extern(C) syscalls) that occupy the calling execution agent
// until the syscall returns, keyed like mir.StdKey; this only sets FactBlock, leaving their other facts unknown.
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

// trustedOpSummaries lists bodiless std fns whose op summary is a complete, hand-verified fact set rather than
// the bodiless default's unknown widening; run's Offloaded is the API's contract, trusted like a declared noalloc.
var trustedOpSummaries = map[string]OpSummary{
	"task.BlockingPool.new": {Known: true, Facts: [numOpFacts]OpFactState{
		FactAlloc: {Maybe: true},
	}},
	"task.BlockingPool.run": {Known: true, Facts: [numOpFacts]OpFactState{
		FactAlloc: {Maybe: true},
		FactBlock: {Maybe: true, Offloaded: true},
	}},
}

// offloadingCallbacks lists bodiless std fns whose fn-typed argument runs on the provider's own worker, never
// synchronously in the caller's frame; callbackEdges marks that argument's EdgeCallback Offloaded so its facts
// merge the same way run's own FactBlock does, instead of being blamed on the caller directly.
var offloadingCallbacks = map[string]bool{
	"task.BlockingPool.run": true,
}

func (r *Result) isOffloadingCallback(fn EntityID) bool {
	return offloadingCallbacks[r.stdKey(fn)]
}

// checkAsyncDirectBlocking flags a direct-blocking operation reachable from an async function's body;
// hostedBlockingProfile's default warns, every other profile name rejects.
func (r *Result) checkAsyncDirectBlocking() {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind != EntFn || e.Flags&EfPoison != 0 {
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
