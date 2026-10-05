package sem

import (
	"strings"

	"kigumi/internal/syntax"
)

// CapabilityUse is one reachable provider op a strict profile can never
// supply; the driver builds these from MIR, since sem runs before MIR exists.
type CapabilityUse struct {
	Op   string
	File FileID
	Node syntax.NodeID
	// Path names the call chain from a canonical root to Op, root first.
	Path []string
}

// Rejects with E1370 instead of leaving a missing Target facility to a native link failure.
func (r *Result) CheckStrictCapabilities(uses []CapabilityUse) {
	for _, u := range uses {
		r.errAt(u.File, u.Node, cStrictCapabilityMissing, u.Op, strings.Join(u.Path, " -> "))
	}
}
