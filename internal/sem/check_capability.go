package sem

import (
	"strings"

	"kigumi/internal/syntax"
)

// The driver builds these from MIR, since sem runs before MIR exists.
type CapabilityUse struct {
	Op         string
	Capability string
	Reason     string
	File       FileID
	Node       syntax.NodeID
	// Path names the call chain from a canonical root to Op, root first.
	Path []string
}

// Rejects with E1370 instead of leaving a missing Target facility to a native link failure.
func (r *Result) CheckCapabilityProvision(uses []CapabilityUse) {
	for _, u := range uses {
		r.errAt(u.File, u.Node, cCapabilityNotProvided, u.Op, u.Capability, u.Reason, strings.Join(u.Path, " -> "))
	}
}
