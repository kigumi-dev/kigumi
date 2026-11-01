package plan

import "kigumi/internal/compiler/sem"

// sem settled the effect summaries before RC planned any of these copies.
func (p *planner) chargeCopies() {
	var sites []sem.CopySite
	facts := paramCopies{}
	for _, f := range p.Funcs {
		if f.Ent == 0 {
			continue
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !p.AllocCopy(f, in) {
					continue
				}
				if fact, ok := p.ParamCopy(f, in); ok {
					facts.add(f.Ent, fact)
					continue
				}
				sites = append(sites, p.CopySite(f, in))
			}
		}
	}
	sites = append(sites, p.chargeCalls(facts)...)
	p.R.ChargeCopies(sites)
}
