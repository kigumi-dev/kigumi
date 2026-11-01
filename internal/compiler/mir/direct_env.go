package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *Program) EnvABI(f *midir.Func) bool {
	if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntClosure {
		return false
	}
	return p.R.DirectClosure(p.R.Types.Closure(f.Ent))
}

func (p *Program) EnvTypes(f *midir.Func) []sem.TypeID {
	return p.R.AggFields(p.R.Types.Closure(f.Ent))
}

// A boxed value is called through its adapter, so only an unboxed one names its body.
func (p *Program) valueCallee(f *midir.Func, in midir.Inst) *midir.Func {
	t := p.WholePos(f, in.Args[0])
	if t == 0 || !p.R.DirectClosure(t) {
		return nil
	}
	return p.ByEnt[p.R.Types.Node(t).Ent]
}

func (p *Program) valueArgPos(f *midir.Func, in midir.Inst, i int) sem.TypeID {
	body := p.valueCallee(f, in)
	if body == nil {
		return 0
	}
	return body.DSig.Param(len(p.EnvTypes(body)) + i - 1)
}
