package reach

import opdef "kigumi/internal/compiler/mir/ops"

func (rs *Reachability) UsesThreads() bool {
	for _, c := range opdef.Capabilities(rs.Ops) {
		if opdef.Capability(c) == opdef.CapTaskThreads {
			return true
		}
	}
	return false
}
