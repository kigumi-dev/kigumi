package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type aggKind uint8

const (
	aggLoose aggKind = iota
	aggRecord
	aggAdt
	aggNone
	aggBox
)

func (p *verifier) aggOf(t sem.TypeID) (aggKind, sem.TypeID) {
	tt := p.R.Types
	t = p.R.UnderlyingRepr(p.DerefAll(t))
	if p.looseType(t) {
		return aggLoose, t
	}
	switch tt.Kind(t) {
	case sem.KClosure:
		return aggRecord, t
	case sem.KIface:
		return aggBox, t
	case sem.KNamed:
		switch p.R.TypeDecl(tt.Node(t).Ent).Form {
		case sem.FormRecord, sem.FormResource:
			return aggRecord, t
		case sem.FormAdt:
			return aggAdt, t
		}
	}
	return aggNone, t
}

func (p *verifier) fieldTypes(t sem.TypeID) ([]sem.TypeID, bool) {
	n := p.R.Types.Node(t)
	if n.Kind == sem.KNamed && len(n.Args) != len(p.R.TypeDecl(n.Ent).Params) {
		return nil, false
	}
	return p.R.AggFields(t), true
}

func (p *verifier) verifyAggregates(f *midir.Func) *Error {
	defs := yieldDefs(f)
	for bid, blk := range f.Blocks {
		bid := midir.BlockID(bid)
		for i, in := range blk.Insts {
			if msg := p.aggMismatch(f, defs, in); msg != "" {
				return fail("aggregates", bid, i, "%s %s", in.Op, msg)
			}
		}
	}
	return nil
}

func (p *verifier) aggMismatch(f *midir.Func, defs cellDefs, in midir.Inst) string {
	ty := func(l midir.LocalID) sem.TypeID { return f.Locals[l].Type }
	switch in.Op {
	case midir.OpField, midir.OpFieldMove, midir.OpSetField:
		if msg := p.fieldMismatch(in, ty(in.Args[0]), ty(in.Args[len(in.Args)-1]), ty(in.Dst)); msg != "" {
			return msg
		}
		if in.Op == midir.OpFieldMove && p.R.IsCopy(ty(in.Dst)) && !p.looseType(ty(in.Dst)) && p.R.Types.Kind(ty(in.Dst)) != sem.KParam {
			return "moves " + p.R.TypeString(ty(in.Dst)) + ", a Copy type, which a read takes"
		}
		if in.Op == midir.OpFieldMove && p.directlyLent(f, in.Args[0]) {
			return "moves a field out of %" + f.Locals[in.Args[0]].Name + ", which the caller still owns"
		}
	case midir.OpPayload, midir.OpTag, midir.OpIsVariant:
		return p.adtMismatch(in, ty(in.Args[0]), ty(in.Dst))
	case midir.OpVariant:
		return p.variantMismatch(f, in)
	case midir.OpRecord:
		return p.recordMismatch(f, in)
	case midir.OpArray:
		return p.arrayMismatch(f, in)
	case midir.OpIndex, midir.OpSetIndex:
		return p.indexMismatch(f, in)
	case midir.OpUnbox, midir.OpIsType:
		if t := p.DerefAll(ty(in.Args[0])); !p.looseType(t) && p.R.Types.Kind(t) != sem.KIface {
			return "reads " + p.R.TypeString(ty(in.Args[0])) + ", which is not a boxed value"
		}
	case midir.OpCellGet, midir.OpCellTake:
		if ok, c := p.cellFits(f, defs, in.Args[0], ty(in.Dst)); !ok {
			return "takes " + p.R.TypeString(ty(in.Dst)) + " out of a cell of " + p.R.TypeString(c)
		}
	case midir.OpCellSet:
		if ok, c := p.cellFits(f, defs, in.Args[0], ty(in.Args[1])); !ok {
			return "stores " + p.R.TypeString(ty(in.Args[1])) + " in a cell of " + p.R.TypeString(c)
		}
	}
	return ""
}

// cellDefs indexes a body's instructions by the local they define, so finding a cell's content does not rescan the body.
type cellDefs map[midir.LocalID][]midir.Inst

func yieldDefs(f *midir.Func) cellDefs {
	defs := cellDefs{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() {
				defs[in.Dst] = append(defs[in.Dst], in)
			}
		}
	}
	return defs
}
