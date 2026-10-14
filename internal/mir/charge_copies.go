package mir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

func (p *Program) allocCopy(f *Func, in Inst) bool {
	if _, ok := p.elemCopy(f, in); ok {
		return true
	}
	if in.Op != OpShare || !in.Copy || !f.DirectPlanned() || f.IsDirect(in.Args[0]) {
		return false
	}
	return in.Own || !f.Locals[in.Args[0]].Cell && p.R.CopyAllocates(f.Locals[in.Args[0]].Type)
}

// elemOps are the std ops that hand out a copy of an array element inside an Option.
var elemOps = map[string]bool{"ARRAY_GET": true, "ARRAY_FIRST": true, "ARRAY_LAST": true}

func (p *Program) elemCopy(f *Func, in Inst) (sem.TypeID, bool) {
	if in.Op != OpCall || in.DPrim != DPNone || !f.DirectPlanned() || !p.IsStdPrimitive(in.Ent) {
		return 0, false
	}
	if id, ok := in.StdOp(); !ok || !elemOps[StdOpNames[id]] {
		return 0, false
	}
	elem, ok := p.R.Types.IsOption(f.Locals[in.Dst].Type)
	return elem, ok
}

func (p *Program) copiedType(f *Func, in Inst) sem.TypeID {
	if elem, ok := p.elemCopy(f, in); ok {
		if p.R.CopyAllocates(elem) {
			return elem
		}
		return f.Locals[in.Dst].Type
	}
	return f.Locals[in.Args[0]].Type
}

func (p *Program) InstAllocates(f *Func, in Inst) bool {
	if in.Op == OpBoxDirect || p.boxedResult(f, in) != 0 {
		return true
	}
	_, deferred := p.paramCopy(f, in)
	return p.allocCopy(f, in) && !deferred
}

// sem settled the effect summaries before PlanRC planned any of these copies.
func (p *Program) chargeCopies() {
	var sites []sem.CopySite
	facts := paramCopies{}
	for _, f := range p.Funcs {
		if f.Ent == 0 {
			continue
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !p.allocCopy(f, in) {
					continue
				}
				if k, ok := p.paramCopy(f, in); ok {
					facts.add(f.Ent, k)
					continue
				}
				sites = append(sites, p.copySite(f, in))
			}
		}
	}
	sites = append(sites, p.chargeCalls(facts)...)
	p.R.ChargeCopies(sites)
}

func (p *Program) copySite(f *Func, in Inst) sem.CopySite {
	elem, isElem := p.elemCopy(f, in)
	copies := !isElem || p.R.CopyAllocates(elem)
	return sem.CopySite{Owner: f.Ent, File: f.File, Node: p.siteNode(f, in), Type: p.copiedType(f, in), Held: !in.Own && !isElem, Elem: isElem && copies, Boxed: isElem && !copies}
}

func (p *Program) siteNode(f *Func, in Inst) syntax.NodeID {
	if in.Node != 0 {
		return in.Node
	}
	return p.R.Entity(f.Ent).Node
}
