package reach

import (
	"sort"

	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

type CapabilityReason struct {
	Capability string `json:"capability"`
	// Op is the std op that draws on Capability; empty when RequiredBy implies it.
	Op         string `json:"op,omitempty"`
	RequiredBy string `json:"requiredBy,omitempty"`
	// RootKind and Path describe the first reaching site, Sites[0].
	RootKind string           `json:"rootKind"`
	Path     []string         `json:"path"`
	Sites    []CapabilitySite `json:"sites"`
	Provided bool             `json:"provided"`
	Why      string           `json:"why,omitempty"`
}

// CapabilitySite is one function that reaches the op, with how many of its call sites do.
type CapabilitySite struct {
	RootKind string   `json:"rootKind"`
	Path     []string `json:"path"`
	Calls    int      `json:"calls"`
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
		sites := rs.capabilitySites(r, op)
		add := func(cap opdef.Capability, name, by string) {
			got := plat.Provides(cap)
			out = append(out, CapabilityReason{Capability: string(cap), Op: name, RequiredBy: by, RootKind: sites[0].RootKind, Path: sites[0].Path, Sites: sites, Provided: got.Provided, Why: got.Reason})
		}
		add(c, opdef.StdOpNames[op], "")
		implied := map[opdef.Capability]bool{c: true}
		opdef.AddRequirements(implied)
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

func (rs *Reachability) capabilitySites(r *sem.Result, op opdef.StdOpId) []CapabilitySite {
	var out []CapabilitySite
	index := map[sem.EntityID]int{}
	for _, site := range rs.AllOpSites[op] {
		if i, ok := index[site.Caller]; ok {
			out[i].Calls++
			continue
		}
		index[site.Caller] = len(out)
		out = append(out, CapabilitySite{RootKind: rs.rootKindOf(site.Caller), Path: rs.PathNames(r, site.Caller), Calls: 1})
	}
	return out
}

func sortedCapabilities(set map[opdef.Capability]bool) []opdef.Capability {
	out := make([]opdef.Capability, 0, len(set))
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
