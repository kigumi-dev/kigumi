package verify

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *verifier) fieldMismatch(in midir.Inst, base, last, dst sem.TypeID) string {
	kind, t := p.aggOf(base)
	switch kind {
	case aggLoose:
		return ""
	case aggNone, aggBox:
		return "names a field of " + p.R.TypeString(base) + ", which has none"
	case aggAdt:
		return "names a field of " + p.R.TypeString(base) + ", a sum type"
	}
	fields, ok := p.fieldTypes(t)
	if !ok {
		return ""
	}
	if in.Index < 0 || in.Index >= len(fields) {
		return "names field " + itoa(in.Index) + " of " + p.R.TypeString(base) + ", which has " + itoa(len(fields))
	}
	if in.Op == midir.OpSetField && !p.compatible(fields[in.Index], last) {
		return "stores " + p.R.TypeString(last) + " in a field of " + p.R.TypeString(fields[in.Index])
	}
	if in.Op != midir.OpSetField && (!p.compatible(fields[in.Index], dst) || p.scalarAsRef(fields[in.Index], dst)) {
		return "reads a field of " + p.R.TypeString(fields[in.Index]) + " into " + p.R.TypeString(dst)
	}
	return ""
}

func (p *verifier) adtMismatch(in midir.Inst, base, dst sem.TypeID) string {
	kind, t := p.aggOf(base)
	if kind == aggLoose {
		return ""
	}
	if kind != aggAdt {
		return "reads " + p.R.TypeString(base) + ", which is not a sum type"
	}
	variants := p.R.AggVariants(t)
	switch in.Op {
	case midir.OpIsVariant:
		if !slices.Contains(variants, in.Ent) {
			return "tests a variant " + p.R.Entity(in.Ent).Name + " that " + p.R.TypeString(base) + " does not have"
		}
	case midir.OpTag:
		if !p.R.Types.IsInteger(dst) && !p.looseType(dst) {
			return "yields " + p.R.TypeString(dst) + ", not an integer"
		}
	case midir.OpPayload:
		return p.payloadMismatch(in, t, dst)
	}
	return ""
}

func (p *verifier) payloadMismatch(in midir.Inst, t, dst sem.TypeID) string {
	if n := p.R.Types.Node(t); len(n.Args) != len(p.R.TypeDecl(n.Ent).Params) {
		return ""
	}
	seen := false
	for _, v := range p.R.AggVariants(t) {
		pl := p.R.AggPayload(t, v)
		if in.Index < 0 || in.Index >= len(pl) {
			continue
		}
		seen = true
		if p.compatible(pl[in.Index], dst) && !p.scalarAsRef(pl[in.Index], dst) {
			return ""
		}
	}
	if !seen {
		return "reads payload slot " + itoa(in.Index) + ", which no variant of " + p.R.TypeString(t) + " has"
	}
	return "reads a payload of " + p.R.TypeString(t) + " into " + p.R.TypeString(dst)
}

func (p *verifier) variantMismatch(f *midir.Func, in midir.Inst) string {
	kind, t := p.aggOf(in.Type)
	if kind == aggLoose {
		return ""
	}
	if kind != aggAdt || !slices.Contains(p.R.AggVariants(t), in.Ent) {
		return "builds " + p.R.Entity(in.Ent).Name + ", which " + p.R.TypeString(in.Type) + " does not have"
	}
	if n := p.R.Types.Node(t); len(n.Args) != len(p.R.TypeDecl(n.Ent).Params) {
		return ""
	}
	pl := p.R.AggPayload(t, in.Ent)
	if len(pl) != len(in.Args) {
		return "builds " + p.R.Entity(in.Ent).Name + " of " + itoa(len(pl)) + " values from " + itoa(len(in.Args))
	}
	for k, a := range in.Args {
		if !p.compatible(pl[k], f.Locals[a].Type) {
			return "puts " + p.R.TypeString(f.Locals[a].Type) + " in a payload of " + p.R.TypeString(pl[k])
		}
	}
	return ""
}

func (p *verifier) recordMismatch(f *midir.Func, in midir.Inst) string {
	kind, t := p.aggOf(f.Locals[in.Dst].Type)
	if kind == aggLoose || kind == aggBox {
		return ""
	}
	if kind != aggRecord || p.R.Types.Node(t).Ent != in.Ent {
		return "builds " + p.R.Entity(in.Ent).Name + " into " + p.R.TypeString(f.Locals[in.Dst].Type)
	}
	fields, ok := p.fieldTypes(t)
	if !ok || len(fields) != len(in.Args) {
		return ""
	}
	for k, a := range in.Args {
		if !p.compatible(fields[k], f.Locals[a].Type) {
			return "puts " + p.R.TypeString(f.Locals[a].Type) + " in a field of " + p.R.TypeString(fields[k])
		}
	}
	return ""
}

func (p *verifier) arrayMismatch(f *midir.Func, in midir.Inst) string {
	tt := p.R.Types
	t := p.R.UnderlyingRepr(f.Locals[in.Dst].Type)
	if p.looseType(t) {
		return ""
	}
	n := tt.Node(t)
	elem, length := sem.TypeID(0), int64(-1)
	switch fixed, count, ok := p.R.FixedElem(t); {
	case ok:
		elem, length = fixed, count
	case n.Kind == sem.KNamed && n.Ent == tt.ArrayEnt() && len(n.Args) == 1:
		elem = n.Args[0]
	default:
		return "builds " + p.R.TypeString(f.Locals[in.Dst].Type) + ", which is not an array"
	}
	if length >= 0 && int(length) != len(in.Args) {
		return "builds " + p.R.TypeString(f.Locals[in.Dst].Type) + " of " + itoa(len(in.Args)) + " elements"
	}
	for _, a := range in.Args {
		if !p.compatibleGeneric(elem, f.Locals[a].Type) {
			return "puts " + p.R.TypeString(f.Locals[a].Type) + " in an array of " + p.R.TypeString(elem)
		}
	}
	return ""
}

func (p *verifier) scalarAsRef(want, got sem.TypeID) bool {
	return p.R.Types.Kind(got) == sem.KRef && p.R.Types.Kind(want) != sem.KRef && p.R.RegScalar(want)
}
