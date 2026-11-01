package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Method values have no body to clone, but their captured receiver types still need minting.
func (s *specializer) withMethodValues(all []sem.EntityID, g *midir.Func) []sem.EntityID {
	seen := map[sem.EntityID]bool{}
	for _, c := range all {
		seen[c] = true
	}
	out := all
	scan := func(f *midir.Func) {
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

func (s *specializer) bodyLocals(c sem.EntityID) []midir.Local {
	if body := s.p.ByEnt[c]; body != nil {
		return body.Locals
	}
	return nil
}

func (s *specializer) envLocals(c sem.EntityID) []midir.Local {
	if body := s.p.ByEnt[c]; body != nil {
		return body.Locals
	}
	var out []midir.Local
	for _, cp := range s.p.R.Closure(c).Captures {
		out = append(out, midir.Local{Type: s.p.R.Entity(cp.Local).Type})
	}
	return out
}
