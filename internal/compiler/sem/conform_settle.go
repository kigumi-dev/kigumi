package sem

// settleBoxConformances asks each existential coercion's conformance at its resolved source type: inference
// asked it while an argument was still a variable, so the witnesses it recorded give way to the settled ones.
func (r *Result) settleBoxConformances() {
	for f := range r.Files {
		fi := r.Files[f]
		for n, co := range fi.Coerce {
			for _, s := range co.Steps {
				from, to := co.From, s.To
				switch s.Kind {
				case CoExistential:
				case CoLendExistential:
					from, to = r.Types.Node(from).Elem, r.Types.Node(to).Elem
				default:
					continue
				}
				if from == TyPoison || to == TyPoison || r.Types.ContainsVar(from) {
					continue
				}
				if res := r.conforms(from, to, fi.Pkg); res.ok {
					if _, had := fi.CoerceWitness[n]; had {
						fi.CoerceWitness[n] = res.witnesses
					}
				}
			}
		}
	}
}
