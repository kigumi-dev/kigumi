package mir

import "kigumi/internal/sem"

// A direct copy is bitwise, so only a boxed one allocates.
func ownCopy(f *Func, in Inst) bool {
	return in.Op == OpShare && in.Own && f.DirectPlanned() && !f.IsDirect(in.Args[0])
}

// sem settled the effect summaries before PlanRC planned any of these copies.
func (p *Program) chargeCopies() {
	var sites []sem.CopySite
	for _, f := range p.Funcs {
		if f.Ent == 0 {
			continue
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !ownCopy(f, in) {
					continue
				}
				n := in.Node
				if n == 0 {
					n = p.R.Entity(f.Ent).Node
				}
				sites = append(sites, sem.CopySite{Owner: f.Ent, File: f.File, Node: n, Type: f.Locals[in.Args[0]].Type})
			}
		}
	}
	p.R.ChargeCopies(sites)
}
