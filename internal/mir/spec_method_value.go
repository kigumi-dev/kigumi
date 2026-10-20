package mir

import "kigumi/internal/sem"

// Method values have no body to clone, but their captured receiver types still need minting.
func (s *specializer) withMethodValues(all []sem.EntityID, g *Func) []sem.EntityID {
	seen := map[sem.EntityID]bool{}
	for _, c := range all {
		seen[c] = true
	}
	out := all
	scan := func(f *Func) {
		for _, l := range f.Locals {
			for _, c := range s.closureTypes(l.Type, nil, 0) {
				if !seen[c] && s.p.R.Closure(c).IsMethodValue() {
					seen[c] = true
					out = append(out, c)
				}
			}
		}
	}
	scan(g)
	for _, c := range all {
		scan(s.p.ByEnt[c])
	}
	return out
}

func (s *specializer) bodyLocals(c sem.EntityID) []Local {
	if body := s.p.ByEnt[c]; body != nil {
		return body.Locals
	}
	return nil
}

func (s *specializer) envLocals(c sem.EntityID) []Local {
	if body := s.p.ByEnt[c]; body != nil {
		return body.Locals
	}
	var out []Local
	for _, cp := range s.p.R.Closure(c).Captures {
		out = append(out, Local{Type: s.p.R.Entity(cp.Local).Type})
	}
	return out
}
