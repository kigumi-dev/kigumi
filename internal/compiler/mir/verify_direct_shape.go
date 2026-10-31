package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

func (p *verifier) directRecord(t sem.TypeID) bool {
	return p.R.DirectAgg(t) && !p.R.DirectAdt(t) && !p.R.DirectClosure(t) && !p.R.DirectFixed(t)
}

func (p *verifier) directShape(f *midir.Func, in midir.Inst) string {
	switch in.Op {
	case midir.OpRecord, midir.OpClosure:
		t := p.wholePos(f, in.Dst)
		kind := p.directRecord(t)
		if in.Op == midir.OpClosure {
			kind = p.R.DirectClosure(t)
		}
		if t != 0 && (!kind || len(in.Args) != len(p.R.AggFields(t))) {
			return "builds a value its type does not lay out"
		}
	case midir.OpVariant:
		t := p.wholePos(f, in.Dst)
		if t != 0 && (!p.R.DirectAdt(t) || !slices.Contains(p.R.AggVariants(t), in.Ent) || len(in.Args) != len(p.R.AggPayload(t, in.Ent))) {
			return "builds a variant its type does not have"
		}
	case midir.OpField, midir.OpFieldMove, midir.OpSetField:
		t := p.wholePos(f, in.Args[0])
		if t != 0 && (!p.directRecord(t) || in.Index < 0 || in.Index >= len(p.R.AggFields(t))) {
			return "names a field its base does not have"
		}
	case midir.OpPayload:
		t := p.wholePos(f, in.Args[0])
		if t != 0 && (!p.R.DirectAdt(t) || in.Index < 0 || in.Index >= p.payloadSlots(t)) {
			return "reads a payload slot its base does not have"
		}
	case midir.OpIsVariant:
		t := p.wholePos(f, in.Args[0])
		if t != 0 && (!p.R.DirectAdt(t) || !slices.Contains(p.R.AggVariants(t), in.Ent)) {
			return "tests a variant its base does not have"
		}
	case midir.OpCallValue:
		if t := p.wholePos(f, in.Args[0]); t != 0 && !p.R.DirectClosure(t) {
			return "calls a value that is not a closure"
		}
	case midir.OpCall:
		if in.DPrim != opdef.DPNone && !p.primTypesFit(f, in, in.DPrim) {
			return "hands a direct primitive operands it does not take"
		}
	case midir.OpBuiltin:
		if protoBuiltin(in) {
			return p.protoShape(f, in)
		}
		if t := p.wholePos(f, in.Dst); fixedFill(in) && t != 0 && (!p.R.DirectFixed(t) || len(in.Args) != 2) {
			return "fills a value that is not an inline array"
		}
		if iterBuiltin(in) && len(in.Args) > 0 && p.fixedBase(f, in.Args[0]) != 0 && (len(in.Args) == 2) == (in.Str == "iter.len") {
			return "walks an inline array with the wrong operands"
		}
	}
	return ""
}

func (p *verifier) payloadSlots(t sem.TypeID) int {
	n := 0
	for _, v := range p.R.AggVariants(t) {
		n = max(n, len(p.R.AggPayload(t, v)))
	}
	return n
}
