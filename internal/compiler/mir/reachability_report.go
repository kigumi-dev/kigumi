package mir

import "kigumi/internal/compiler/sem"

// ReasonEntry is one reachable entity in Reachability.Order, a plain
// value a build plan or `kigumi build --reason-graph` can marshal.
type ReasonEntry struct {
	Name   string `json:"name"`
	Root   string `json:"root,omitempty"`
	Via    string `json:"via,omitempty"`
	Caller string `json:"caller,omitempty"`
}

var rootKindNames = map[RootKind]string{
	RootEntry:    "entry",
	RootExport:   "export",
	RootTest:     "test",
	RootDropGlue: "drop-glue",
}

// A requirement/provision reason graph at the function-entity grain.
func (rs *Reachability) Report(r *sem.Result) []ReasonEntry {
	out := make([]ReasonEntry, 0, len(rs.Order))
	for _, id := range rs.Order {
		reason := rs.Reason[id]
		e := ReasonEntry{Name: r.Entity(id).Name}
		if reason.Root != RootNone {
			e.Root = rootKindNames[reason.Root]
		} else {
			e.Via, e.Caller = reason.Via, r.Entity(reason.Caller).Name
		}
		out = append(out, e)
	}
	return out
}
