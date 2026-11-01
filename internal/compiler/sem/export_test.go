package sem

// RootForwardFrames reruns the root-bridge reach and counts its calleeHopsWith frames.
func RootForwardFrames(r *Result) int {
	runner := r.rootRunner()
	if runner == 0 {
		return 0
	}
	return r.inferRootRun(r.effectNodes(), runner).forwardFrames
}
