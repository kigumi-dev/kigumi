package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

func (p *Program) AllocCopy(f *midir.Func, in midir.Inst) bool {
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

func (p *Program) CopiedType(f *midir.Func, in midir.Inst) sem.TypeID {
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
	if in.Op == midir.OpBoxDirect && !in.Fixed || p.BoxedResult(f, in) != 0 {
		return true
	}
	_, deferred := p.ParamCopy(f, in)
	return p.AllocCopy(f, in) && !deferred
}

func (p *Program) CopySite(f *midir.Func, in midir.Inst) sem.CopySite {
	if t, ok := p.contentCopy(f, in); ok {
		return sem.CopySite{Owner: f.Ent, File: f.File, Node: p.SiteNode(f, in), Type: t, Content: true}
	}
	elem, isElem := p.elemCopy(f, in)
	copies := !isElem || p.R.CopyAllocates(elem)
	return sem.CopySite{Owner: f.Ent, File: f.File, Node: p.SiteNode(f, in), Type: p.CopiedType(f, in), Held: !in.Own && !isElem, Elem: isElem && copies, Boxed: isElem && !copies}
}

func (p *Program) SiteNode(f *midir.Func, in midir.Inst) syntax.NodeID {
	if in.Node != 0 {
		return in.Node
	}
	return p.R.Entity(f.Ent).Node
}
