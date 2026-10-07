package mir

import (
	"sort"

	"kigumi/internal/sem"
)

type CapabilityReason struct {
	Capability string `json:"capability"`
	// Op is the std op that draws on Capability; empty when RequiredBy implies it.
	Op         string   `json:"op,omitempty"`
	RequiredBy string   `json:"requiredBy,omitempty"`
	RootKind   string   `json:"rootKind"`
	Path       []string `json:"path"`
	Provided   bool     `json:"provided"`
	Why        string   `json:"why,omitempty"`
}

func (cr CapabilityReason) Root() string {
	if len(cr.Path) == 0 {
		return ""
	}
	return cr.Path[0]
}

func (rs *Reachability) CapabilityReasons(r *sem.Result, plat Platform) []CapabilityReason {
	var out []CapabilityReason
	for _, op := range rs.Ops {
		c, ok := op.Capability()
		if !ok {
			continue
		}
		site := rs.OpSites[op]
		path := rs.PathNames(r, site.Caller)
		kind := rs.rootKindOf(site.Caller)
		add := func(cap Capability, name, by string) {
			got := plat.Provides(cap)
			out = append(out, CapabilityReason{Capability: string(cap), Op: name, RequiredBy: by, RootKind: kind, Path: path, Provided: got.Provided, Why: got.Reason})
		}
		add(c, StdOpNames[op], "")
		implied := map[Capability]bool{c: true}
		closure(implied)
		delete(implied, c)
		for _, need := range sortedCapabilities(implied) {
			add(need, "", string(c))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Capability != out[j].Capability {
			return out[i].Capability < out[j].Capability
		}
		return out[i].Op < out[j].Op
	})
	return out
}

func sortedCapabilities(set map[Capability]bool) []Capability {
	out := make([]Capability, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (rs *Reachability) rootKindOf(ent sem.EntityID) string {
	seen := map[sem.EntityID]bool{}
	for ent != 0 && !seen[ent] {
		seen[ent] = true
		reason, ok := rs.Reason[ent]
		if !ok {
			break
		}
		if reason.Root != RootNone {
			return rootKindNames[reason.Root]
		}
		ent = reason.Caller
	}
	return ""
}
