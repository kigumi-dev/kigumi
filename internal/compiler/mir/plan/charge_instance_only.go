package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// charged leaves out a body without an entity and a generic body that runs only as its instances.
func (p *planner) charged(f *midir.Func) bool { return f.Ent != 0 && !p.InstanceOnly[f.Ent] }

// chargeGenerics also charges an instance's copy to its generic when only instances run that generic, since
// callers' summaries name the generic.
func (p *planner) chargeGenerics(sites []sem.CopySite) []sem.CopySite {
	out := append([]sem.CopySite(nil), sites...)
	for _, s := range sites {
		if gen := p.genericOf(s.Owner); s.Erased == 0 && gen != s.Owner && p.InstanceOnly[gen] {
			s.Owner = gen
			out = append(out, s)
		}
	}
	return out
}

func (p *planner) genericOf(ent sem.EntityID) sem.EntityID {
	if in, ok := p.R.InstanceOf(ent); ok {
		return in.Gen
	}
	return p.R.ClosureOrigin(ent)
}
