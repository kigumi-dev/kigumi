package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

func (p *Program) allocCopy(f *midir.Func, in midir.Inst) bool {
	if _, ok := p.elemCopy(f, in); ok {
		return true
	}
	if _, ok := p.contentCopy(f, in); ok {
		return true
	}
	if in.Op != midir.OpShare || !in.Copy || !f.DirectPlanned() || f.IsDirect(in.Args[0]) || p.copiesInPlace(f, in) {
		return false
	}
	return in.Own || !f.Locals[in.Args[0]].Cell && p.R.CopyAllocates(f.Locals[in.Args[0]].Type)
}

// elemOps are the std ops that hand out a copy of an array element inside an Option.
var elemOps = func() map[string]bool {
	m := map[string]bool{}
	for _, op := range opdef.Registry.Ops {
		m[op.Name] = op.Elem
	}
	return m
}()

func (p *Program) elemCopy(f *midir.Func, in midir.Inst) (sem.TypeID, bool) {
	if in.Op != midir.OpCall || in.DPrim != opdef.DPNone || !f.DirectPlanned() || !ownquery.IsStdPrimitive(p.R, in.Ent) {
		return 0, false
	}
	if id, ok := in.StdOp(); !ok || !elemOps[opdef.StdOpNames[id]] {
		return 0, false
	}
	elem, ok := p.R.Types.IsOption(f.Locals[in.Dst].Type)
	return elem, ok
}

func (p *Program) copiedType(f *midir.Func, in midir.Inst) sem.TypeID {
	if t, ok := p.contentCopy(f, in); ok {
		return t
	}
	if elem, ok := p.elemCopy(f, in); ok {
		if p.R.CopyAllocates(elem) {
			return elem
		}
		return f.Locals[in.Dst].Type
	}
	return f.Locals[in.Args[0]].Type
}

func (p *Program) InstAllocates(f *midir.Func, in midir.Inst) bool {
	if in.Op == midir.OpBoxDirect && !in.Fixed || p.boxedResult(f, in) != 0 {
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
				if fact, ok := p.paramCopy(f, in); ok {
					facts.add(f.Ent, fact)
					continue
				}
				sites = append(sites, p.copySite(f, in))
			}
		}
	}
	sites = append(sites, p.chargeCalls(facts)...)
	p.R.ChargeCopies(sites)
}

func (p *Program) copySite(f *midir.Func, in midir.Inst) sem.CopySite {
	if t, ok := p.contentCopy(f, in); ok {
		return sem.CopySite{Owner: f.Ent, File: f.File, Node: p.siteNode(f, in), Type: t, Content: true}
	}
	elem, isElem := p.elemCopy(f, in)
	copies := !isElem || p.R.CopyAllocates(elem)
	return sem.CopySite{Owner: f.Ent, File: f.File, Node: p.siteNode(f, in), Type: p.copiedType(f, in), Held: !in.Own && !isElem, Elem: isElem && copies, Boxed: isElem && !copies}
}

func (p *Program) siteNode(f *midir.Func, in midir.Inst) syntax.NodeID {
	if in.Node != 0 {
		return in.Node
	}
	return p.R.Entity(f.Ent).Node
}
