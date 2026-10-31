package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	mirlayout "kigumi/internal/compiler/mir/layout"
	"kigumi/internal/compiler/sem"
)

type aggField struct {
	T    sem.TypeID
	Ty   Type
	Mem  Type
	Nk   int
	Agg  *aggInfo
	Unit bool
	// Ref holds one counted reference as an i64, so the layout stays the same on every target.
	Ref bool
}

func (f aggField) mem() Type {
	if f.Mem != TVoid {
		return f.Mem
	}
	return f.Ty
}

// Payload slots are shared across variants, so slot i has the same offset whichever variant is live.
type aggVariant struct {
	Ent     sem.EntityID
	Payload []aggField
}

// Size and Align are target-independent: no leaf is pointer-width.
type aggInfo struct {
	T        sem.TypeID
	Label    string
	Name     string
	Ent      sem.EntityID
	Fields   []aggField
	Variants []aggVariant
	Slots    []int64
	Pay      string
	Size     int64
	Align    int64
	Clo      *midir.Func
	Arr      *aggField
	Len      int64
}

func (a *aggInfo) isAdt() bool { return a.Variants != nil }

// aggOrder keeps nested types ahead of the types that embed them.
func (e *emitter) agg(t sem.TypeID) (*aggInfo, bool) {
	if a, ok := e.aggs[t]; ok {
		return a, a != nil
	}
	if !e.r.DirectAgg(t) {
		e.aggs[t] = nil
		return nil, false
	}
	adt := e.r.DirectAdt(t)
	ent := e.r.Types.Node(t).Ent
	a := &aggInfo{T: t, Ent: ent, Align: 1}
	spelling, prefix := string(e.r.TypeKey(t)), "agg"
	switch {
	case adt:
		prefix = "adt"
	case e.r.DirectClosure(t):
		a.Clo = e.p.ByEnt[ent]
		spelling, prefix = a.Clo.Name, "clo"
	}
	if _, _, ok := e.r.FixedElem(t); ok {
		prefix = "arr"
	}
	a.Label = e.aggLabel(spelling)
	a.Name = "%\"kg." + prefix + "." + a.Label + "\""
	e.aggs[t] = a
	switch {
	case adt:
		e.layoutAdt(t, a)
	case prefix == "arr":
		e.layoutFixed(t, a)
	default:
		e.layoutRecord(t, a)
	}
	if size, align := mirlayout.AggLayout(e.r, t); size != a.Size || align != a.Align {
		panic(fmt.Sprintf("llgen: %s lays out as %d/%d bytes, MIR sized it %d/%d", a.Name, a.Size, a.Align, size, align))
	}
	e.aggOrder = append(e.aggOrder, t)
	return a, true
}

// Two packages may declare a type of the same spelling, so a later one takes a numbered label.
func (e *emitter) aggLabel(s string) string {
	if e.aggLabels == nil {
		e.aggLabels = map[string]bool{}
	}
	label := s
	for i := 2; e.aggLabels[label]; i++ {
		label = fmt.Sprintf("%s#%d", s, i)
	}
	e.aggLabels[label] = true
	return label
}

func (e *emitter) layoutRecord(t sem.TypeID, a *aggInfo) {
	for _, ft := range e.r.AggFields(t) {
		f := e.leafField(ft)
		s, al := fieldBytes(f)
		a.Size = roundUp(a.Size, al) + s
		a.Align = maxI64(a.Align, al)
		a.Fields = append(a.Fields, f)
	}
	a.Size = roundUp(a.Size, a.Align)
}

func (e *emitter) leafField(t sem.TypeID) aggField {
	f := aggField{T: t}
	if t == sem.TyUnit {
		f.Unit = true
		return f
	}
	if sub, ok := e.agg(t); ok {
		f.Agg = sub
		return f
	}
	if e.r.CountedSlot(t) {
		f.Ref = true
		return f
	}
	ty, _ := e.scalarType(t)
	f.Ty, f.Nk = ty, e.numKind(t)
	return f
}

func (f aggField) llType() string {
	switch {
	case f.Unit:
		return "[0 x i8]"
	case f.Agg != nil:
		return f.Agg.Name
	case f.Ref:
		return "i64"
	case f.mem() == TI128:
		return wideSlot
	}
	return f.mem().String()
}

func (a *aggInfo) body() string {
	switch {
	case a.isAdt():
		return adtTag + ", " + a.Pay
	case a.Arr != nil:
		return fmt.Sprintf("[%d x %s]", a.Len, a.Arr.llType())
	}
	parts := make([]string, len(a.Fields))
	for i, f := range a.Fields {
		parts[i] = f.llType()
	}
	return strings.Join(parts, ", ")
}

func (e *emitter) aggTypes() string {
	var sb strings.Builder
	for _, t := range e.aggOrder {
		fmt.Fprintf(&sb, "%s = type { %s }\n", e.aggs[t].Name, e.aggs[t].body())
	}
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	return sb.String()
}
