package driver

import "kigumi/internal/mir"

// CapabilityPath is one Capability a module's reachable code draws on,
// with the call path from a root to where it first reached that op.
type CapabilityPath struct {
	Name string
	Path []string
}

// Capabilities is a.Prog's reachable Capability vocabulary; nil if a has
// no program (unanalyzed, or diagnostics stopped it before MIR).
func Capabilities(a *Analysis) []string {
	if a == nil || a.Prog == nil {
		return nil
	}
	return mir.Capabilities(mir.Reachable(a.Prog).Ops)
}

// CapabilityReasonGraph is Capabilities, each path-named back to its root.
func CapabilityReasonGraph(a *Analysis) []CapabilityPath {
	if a == nil || a.Prog == nil {
		return nil
	}
	reach := mir.Reachable(a.Prog)
	var out []CapabilityPath
	for _, c := range mir.Capabilities(reach.Ops) {
		for _, op := range reach.Ops {
			got, ok := op.Capability()
			if !ok || string(got) != c {
				continue
			}
			site := reach.OpSites[op]
			out = append(out, CapabilityPath{Name: c, Path: reach.PathNames(a.Res, site.Caller)})
			break
		}
	}
	return out
}
