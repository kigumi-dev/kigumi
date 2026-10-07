package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/sem"
)

type aggField struct {
	T    sem.TypeID
	Ty   Type
	Nk   int
	Agg  *aggInfo
	Unit bool
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
}

func (a *aggInfo) isAdt() bool { return a.Variants != nil }

func scalarBytes(t Type) int64 {
	switch t {
	case TI1:
		return 1
	case TI32:
		return 4
	}
	return 8
}

func roundUp(n, a int64) int64 { return (n + a - 1) / a * a }

func fieldBytes(f aggField) (int64, int64) {
	switch {
	case f.Unit:
		return 0, 1
	case f.Agg != nil:
		return f.Agg.Size, f.Agg.Align
	}
	b := scalarBytes(f.Ty)
	return b, b
}

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
	label := e.aggLabel(t)
	a := &aggInfo{T: t, Label: label, Name: aggName(adt, label), Ent: e.r.Types.Node(t).Ent, Align: 1}
	e.aggs[t] = a
	if adt {
		e.layoutAdt(t, a)
	} else {
		e.layoutRecord(t, a)
	}
	e.aggOrder = append(e.aggOrder, t)
	return a, true
}

// Two packages may declare a type of the same spelling, so a later one takes a numbered label.
func (e *emitter) aggLabel(t sem.TypeID) string {
	s := e.r.TypeString(t)
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

func aggName(adt bool, s string) string {
	if adt {
		return "%\"kg.adt." + s + "\""
	}
	return "%\"kg.agg." + s + "\""
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
	ty, _ := e.scalarType(t)
	f.Ty, f.Nk = ty, e.numKind(t)
	return f
}

func maxI64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (f aggField) llType() string {
	switch {
	case f.Unit:
		return "[0 x i8]"
	case f.Agg != nil:
		return f.Agg.Name
	}
	return f.Ty.String()
}

func (a *aggInfo) body() string {
	if a.isAdt() {
		return adtTag + ", " + a.Pay
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
