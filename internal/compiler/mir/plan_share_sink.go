package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

type sinkKind uint8

const (
	noSink   sinkKind = iota
	takesArg          // stores it or hands it over to an owner
	keepsArg          // it borrows it and may keep a reference of its own
)

func (p *Program) sinkAt(f *midir.Func, in midir.Inst, i int, s *shareSummary) sinkKind {
	switch in.Op {
	case midir.OpRecord, midir.OpVariant, midir.OpArray, midir.OpClosure, midir.OpBox, midir.OpBind, midir.OpGlobalSet:
		return takesArg
	case midir.OpFutureNew:
		return sinkIf(i > 0, keepsArg)
	case midir.OpSetField, midir.OpCellSet:
		return sinkIf(i == 1, takesArg)
	case midir.OpSetIndex:
		return sinkIf(i == 2, takesArg)
	case midir.OpNewCell:
		return sinkIf(f.Locals[in.Dst].Ent != 0, takesArg)
	case midir.OpCallValue:
		return sinkIf(i > 0 && !p.inPlace(in, i), takesArg)
	case midir.OpCall:
		return p.callSink(in, i, s)
	}
	return noSink
}

func sinkIf(ok bool, k sinkKind) sinkKind {
	if ok {
		return k
	}
	return noSink
}

func (p *Program) callSink(in midir.Inst, i int, s *shareSummary) sinkKind {
	if p.R.Entity(in.Ent).Kind != sem.EntFn {
		return noSink
	}
	info := p.R.Fn(in.Ent)
	switch {
	case ownquery.IsStdPrimitive(p.R, in.Ent):
		return sinkIf(i > 0 || info.Recv == sem.RecvNone, keepsArg)
	case info.Abi != "" || info.Naked:
		return noSink
	}
	g := p.ByEnt[in.Ent]
	if g == nil {
		return sinkIf(!p.inPlace(in, i) && !p.lentParam(in, i), takesArg)
	}
	if i >= len(g.Params) || p.lentLocal(g, g.Params[i]) {
		return noSink
	}
	return sinkIf(s.taken[g][i], takesArg)
}
