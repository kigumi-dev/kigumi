package mir

import (
	opdef "kigumi/internal/compiler/mir/ops"
)

func UsesThreads(p *Program) bool {
	for _, c := range opdef.Capabilities(Reachable(p).Ops) {
		if opdef.Capability(c) == opdef.CapTaskThreads {
			return true
		}
	}
	return false
}
