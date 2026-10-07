package mir

import (
	"slices"

	"kigumi/internal/sem"
)

func (p *Program) directRecord(t sem.TypeID) bool {
	return p.R.DirectAgg(t) && !p.R.DirectAdt(t) && !p.R.DirectClosure(t) && !p.R.DirectFixed(t)
}

func (p *Program) directShape(f *Func, in Inst) string {
	switch in.Op {
	case OpRecord, OpClosure:
		t := p.wholePos(f, in.Dst)
		kind := p.directRecord(t)
		if in.Op == OpClosure {
			kind = p.R.DirectClosure(t)
		}
		if t != 0 && (!kind || len(in.Args) != len(p.R.AggFields(t))) {
			return "builds a value its type does not lay out"
		}
	case OpVariant:
		t := p.wholePos(f, in.Dst)
		if t != 0 && (!p.R.DirectAdt(t) || !slices.Contains(p.R.AggVariants(t), in.Ent) || len(in.Args) != len(p.R.AggPayload(t, in.Ent))) {
			return "builds a variant its type does not have"
		}
	case OpField, OpFieldMove, OpSetField:
		t := p.wholePos(f, in.Args[0])
		if t != 0 && (!p.directRecord(t) || in.Index < 0 || in.Index >= len(p.R.AggFields(t))) {
			return "names a field its base does not have"
		}
	case OpPayload:
		t := p.wholePos(f, in.Args[0])
		if t != 0 && (!p.R.DirectAdt(t) || in.Index < 0 || in.Index >= p.payloadSlots(t)) {
			return "reads a payload slot its base does not have"
		}
	case OpIsVariant:
		t := p.wholePos(f, in.Args[0])
		if t != 0 && (!p.R.DirectAdt(t) || !slices.Contains(p.R.AggVariants(t), in.Ent)) {
			return "tests a variant its base does not have"
		}
	case OpCallValue:
		if t := p.wholePos(f, in.Args[0]); t != 0 && !p.R.DirectClosure(t) {
			return "calls a value that is not a closure"
		}
	case OpBuiltin:
		if t := p.wholePos(f, in.Dst); fixedFill(in) && t != 0 && (!p.R.DirectFixed(t) || len(in.Args) != 2) {
			return "fills a value that is not an inline array"
		}
		if iterBuiltin(in) && len(in.Args) > 0 && p.fixedBase(f, in.Args[0]) != 0 && (len(in.Args) == 2) == (in.Str == "iter.len") {
			return "walks an inline array with the wrong operands"
		}
	}
	return ""
}

func (p *Program) payloadSlots(t sem.TypeID) int {
	n := 0
	for _, v := range p.R.AggVariants(t) {
		n = max(n, len(p.R.AggPayload(t, v)))
	}
	return n
}
