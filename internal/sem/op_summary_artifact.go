package sem

// OpFactJSON is one true fact's artifact encoding; a false fact is simply
// absent from OpSummaryJSON.Facts, so the encoding stays small.
type OpFactJSON struct {
	Fact            string `json:"fact"`
	Unknown         bool   `json:"unknown,omitempty"`
	InterruptUnsafe bool   `json:"interruptUnsafe,omitempty"`
	Offloaded       bool   `json:"offloaded,omitempty"`
}

// OpSummaryJSON is one function's cross-package-visible artifact entry. A
// reader on an unrecognized Schema must treat Facts as unknown, not empty.
type OpSummaryJSON struct {
	Schema     int          `json:"schema"`
	Unresolved bool         `json:"unresolved"`
	Facts      []OpFactJSON `json:"facts"`
}

func (r *Result) opSummaryArtifact(id EntityID) OpSummaryJSON {
	s := r.opSummary(id)
	out := OpSummaryJSON{Schema: OpSummarySchemaVersion, Unresolved: s.Unresolved}
	for f := OpFact(0); f < numOpFacts; f++ {
		st := s.Facts[f]
		if !st.Maybe {
			continue
		}
		out.Facts = append(out.Facts, OpFactJSON{Fact: f.String(), Unknown: st.Cause.Unknown, InterruptUnsafe: st.InterruptUnsafe, Offloaded: st.Offloaded})
	}
	return out
}

// PublicOpSummaries is every exported top-level function's artifact entry,
// keyed by "pkg/path.Name".
func (r *Result) PublicOpSummaries() map[string]OpSummaryJSON {
	out := map[string]OpSummaryJSON{}
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind != EntFn || e.Vis.Level != VisPub || e.Flags&EfPoison != 0 {
			continue
		}
		out[r.Packages[e.Pkg].Path+"."+e.Name] = r.opSummaryArtifact(EntityID(id))
	}
	return out
}

// OpSummaryOf is opSummaryArtifact, exported for callers outside the
// package (internal/schema) that build a declaration's contract summary.
func (r *Result) OpSummaryOf(id EntityID) OpSummaryJSON { return r.opSummaryArtifact(id) }

// HasOpFact reports whether j's Facts include name; an unrecognized Schema
// answers true for every name (widen to unknown).
func (j OpSummaryJSON) HasOpFact(name string) bool {
	if j.Schema != OpSummarySchemaVersion {
		return true
	}
	for _, f := range j.Facts {
		if f.Fact == name {
			return true
		}
	}
	return false
}
