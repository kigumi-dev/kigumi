package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/sem"
)

// Agg set means a nested aggregate; otherwise a register scalar.
type aggField struct {
	T   sem.TypeID
	Ty  Type
	Nk  int
	Agg *aggInfo
}

// Size and Align are target-independent: no leaf is pointer-width.
type aggInfo struct {
	T      sem.TypeID
	Name   string
	Ent    sem.EntityID
	Fields []aggField
	Size   int64
	Align  int64
}

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

// aggOrder keeps nested types ahead of the types that embed them.
func (e *emitter) agg(t sem.TypeID) (*aggInfo, bool) {
	if a, ok := e.aggs[t]; ok {
		return a, a != nil
	}
	if !e.r.DirectAgg(t) {
		e.aggs[t] = nil
		return nil, false
	}
	a := &aggInfo{T: t, Name: "%\"kg.agg." + e.r.TypeString(t) + "\"", Ent: e.r.Types.Node(t).Ent, Align: 1}
	e.aggs[t] = a
	for _, ft := range e.r.AggFields(t) {
		f := aggField{T: ft}
		if sub, ok := e.agg(ft); ok {
			f.Agg = sub
			a.Size = roundUp(a.Size, sub.Align) + sub.Size
			a.Align = maxI64(a.Align, sub.Align)
		} else {
			ty, _ := e.scalarType(ft)
			f.Ty, f.Nk = ty, e.numKind(ft)
			b := scalarBytes(ty)
			a.Size = roundUp(a.Size, b) + b
			a.Align = maxI64(a.Align, b)
		}
		a.Fields = append(a.Fields, f)
	}
	a.Size = roundUp(a.Size, a.Align)
	e.aggOrder = append(e.aggOrder, t)
	return a, true
}

func maxI64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (f aggField) llType() string {
	if f.Agg != nil {
		return f.Agg.Name
	}
	return f.Ty.String()
}

func (e *emitter) aggTypes() string {
	var sb strings.Builder
	for _, t := range e.aggOrder {
		a := e.aggs[t]
		parts := make([]string, len(a.Fields))
		for i, f := range a.Fields {
			parts[i] = f.llType()
		}
		fmt.Fprintf(&sb, "%s = type { %s }\n", a.Name, strings.Join(parts, ", "))
	}
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	return sb.String()
}
