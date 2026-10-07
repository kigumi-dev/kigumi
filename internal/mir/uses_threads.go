package mir

func UsesThreads(p *Program) bool {
	for _, c := range Capabilities(Reachable(p).Ops) {
		if Capability(c) == CapTaskThreads {
			return true
		}
	}
	return false
}
