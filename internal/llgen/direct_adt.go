package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// Field 1 is the payload area every variant writes its own slots into.
const adtTag = "i32"
const adtTagBytes = 4

func (e *emitter) layoutAdt(t sem.TypeID, a *aggInfo) {
	for _, v := range e.r.AggVariants(t) {
		av := aggVariant{Ent: v}
		for _, pt := range e.r.AggPayload(t, v) {
			av.Payload = append(av.Payload, e.leafField(pt))
		}
		a.Variants = append(a.Variants, av)
	}
	sizes, aligns := slotBounds(a.Variants)
	cur, pay := int64(0), int64(1)
	a.Slots = make([]int64, len(sizes))
	for j := range sizes {
		cur = roundUp(cur, aligns[j])
		a.Slots[j], cur = cur, cur+sizes[j]
		pay = maxI64(pay, aligns[j])
	}
	words := roundUp(cur, pay) / pay
	a.Pay = fmt.Sprintf("[%d x i%d]", words, pay*8)
	a.Align = maxI64(adtTagBytes, pay)
	a.Size = roundUp(roundUp(adtTagBytes, pay)+words*pay, a.Align)
}

// Slot i takes the widest payload any variant puts there, so a read needs only the slot index.
func slotBounds(vs []aggVariant) (sizes, aligns []int64) {
	for _, v := range vs {
		for j, f := range v.Payload {
			if j >= len(sizes) {
				sizes, aligns = append(sizes, 0), append(aligns, 1)
			}
			s, al := fieldBytes(f)
			sizes[j], aligns[j] = maxI64(sizes[j], s), maxI64(aligns[j], al)
		}
	}
	return sizes, aligns
}

func (a *aggInfo) tagOf(v sem.EntityID) int {
	for i, av := range a.Variants {
		if av.Ent == v {
			return i
		}
	}
	return -1
}

func (e *emitter) tagPtr(sb *strings.Builder, a *aggInfo, base string) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = getelementptr %s, ptr %s, i32 0, i32 0\n", t, a.Name, base)
	return t
}

func (e *emitter) slotPtr(sb *strings.Builder, a *aggInfo, base string, j int) string {
	p := e.newTmp()
	fmt.Fprintf(sb, "  %s = getelementptr %s, ptr %s, i32 0, i32 1\n", p, a.Name, base)
	if a.Slots[j] == 0 {
		return p
	}
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = getelementptr i8, ptr %s, i64 %d\n", t, p, a.Slots[j])
	return t
}

func (e *emitter) buildVariant(sb *strings.Builder, a *aggInfo, in mir.Inst) {
	base := aggPtr(in.Dst)
	zeroAgg(sb, a, base)
	fmt.Fprintf(sb, "  store %s %d, ptr %s\n", adtTag, a.tagOf(in.Ent), e.tagPtr(sb, a, base))
	for j, f := range a.Variants[a.tagOf(in.Ent)].Payload {
		e.storeLeaf(sb, f, e.slotPtr(sb, a, base, j), in.Args[j])
	}
}

func (e *emitter) readPayload(sb *strings.Builder, f *mir.Func, a *aggInfo, in mir.Inst) {
	if f.Locals[in.Dst].Type == sem.TyUnit {
		e.store(sb, in.Dst, e.irb(sb).Call(rtUnit).Text)
		return
	}
	t := f.Locals[in.Dst].Type
	ty, _ := e.scalarType(e.r.UnderlyingRepr(t))
	fld := aggField{T: t, Ty: ty, Nk: e.numKind(t), Agg: e.direct[in.Dst]}
	e.loadLeaf(sb, f, fld, e.slotPtr(sb, a, aggPtr(in.Args[0]), in.Index), in.Dst)
}

func (e *emitter) testVariant(sb *strings.Builder, f *mir.Func, a *aggInfo, in mir.Inst) {
	tag := e.newTmp()
	fmt.Fprintf(sb, "  %s = load %s, ptr %s\n", tag, adtTag, e.tagPtr(sb, a, fmt.Sprintf("%%l%d", in.Args[0])))
	c := e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp eq %s %s, %d\n", c, adtTag, tag, a.tagOf(in.Ent))
	e.storeScalarResult(sb, f, in.Dst, Value{TI1, c})
}
