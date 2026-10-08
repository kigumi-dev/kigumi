package mir

import "kigumi/internal/sem"

func (p *Program) allocCopy(f *Func, in Inst) bool {
	if _, ok := p.elemCopy(f, in); ok {
		return true
	}
	if in.Op != OpShare || !in.Copy || !f.DirectPlanned() || f.IsDirect(in.Args[0]) {
		return false
	}
	return in.Own || !f.Locals[in.Args[0]].Cell && p.R.CopyAllocates(f.Locals[in.Args[0]].Type)
}

// Array.get copies its element into the Option it returns unless the call took the direct form;
// a generic element is left alone, since the body may be instantiated with a scalar.
func (p *Program) elemCopy(f *Func, in Inst) (sem.TypeID, bool) {
	if in.Op != OpCall || in.DPrim != DPNone || !f.DirectPlanned() || !p.IsStdPrimitive(in.Ent) {
		return 0, false
	}
	if id, ok := in.StdOp(); !ok || StdOpNames[id] != "ARRAY_GET" {
		return 0, false
	}
	elem, ok := p.R.Types.IsOption(f.Locals[in.Dst].Type)
	return elem, ok && len(p.R.Types.Params(elem)) == 0 && p.R.CopyAllocates(elem)
}

func (p *Program) copiedType(f *Func, in Inst) sem.TypeID {
	if t, ok := p.elemCopy(f, in); ok {
		return t
	}
	return f.Locals[in.Args[0]].Type
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
				if !p.allocCopy(f, in) {
					continue
				}
				n := in.Node
				if n == 0 {
					n = p.R.Entity(f.Ent).Node
				}
				_, elem := p.elemCopy(f, in)
				sites = append(sites, sem.CopySite{Owner: f.Ent, File: f.File, Node: n, Type: p.copiedType(f, in), Held: !in.Own && !elem, Elem: elem})
			}
		}
	}
	p.R.ChargeCopies(sites)
}
