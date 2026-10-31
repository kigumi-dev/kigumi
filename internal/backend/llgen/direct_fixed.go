package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/sem"
)

// A FixedArray is one struct field [N x elem]; a scalar element is stored at its own width.
func (e *emitter) layoutFixed(t sem.TypeID, a *aggInfo) {
	elem, n, _ := e.r.FixedElem(t)
	f := e.leafField(elem)
	if f.Agg == nil && !f.Unit {
		f.Mem = e.memType(elem, f.Ty)
	}
	a.Arr, a.Len = &f, n
	s, al := fieldBytes(f)
	a.Align = al
	a.Size = roundUp(s, al) * n
}

func (e *emitter) memType(t sem.TypeID, reg Type) Type {
	if !e.r.Types.IsInteger(t) {
		return reg
	}
	switch e.r.Types.Width(t) {
	case 8:
		return TI8
	case 16:
		return TI16
	case 32:
		return TI32
	}
	return reg
}

// An array argument would split into one register or stack slot per element.
func (a *aggInfo) holdsArray() bool {
	if a.Arr != nil {
		return true
	}
	for _, f := range a.leaves() {
		if f.Agg != nil && f.Agg.holdsArray() {
			return true
		}
	}
	return false
}

func (e *emitter) elemAt(sb *strings.Builder, a *aggInfo, base, idx string) string {
	g := e.newTmp()
	fmt.Fprintf(sb, "  %s = getelementptr %s, ptr %s, i32 0, i32 0, i64 %s\n", g, a.Name, base, idx)
	return g
}

func (e *emitter) countLoop(sb *strings.Builder, n int64, body func(j string)) {
	pre, head, step, latch, done := e.label("each"), e.label("each"), e.label("each"), e.label("each"), e.label("each")
	j, c, k := e.newTmp(), e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  br label %%%s\n%s:\n  br label %%%s\n%s:\n", pre, pre, head, head)
	fmt.Fprintf(sb, "  %s = phi i64 [ 0, %%%s ], [ %s, %%%s ]\n", j, pre, k, latch)
	fmt.Fprintf(sb, "  %s = icmp ult i64 %s, %d\n  br i1 %s, label %%%s, label %%%s\n%s:\n", c, j, n, c, step, done, step)
	body(j)
	fmt.Fprintf(sb, "  br label %%%s\n%s:\n  %s = add i64 %s, 1\n  br label %%%s\n%s:\n", latch, latch, k, j, head, done)
}

// The boxed form is an Array of exactly N elements, grown by push so a long one needs no stack buffer.
func (e *emitter) arrBoxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal ptr %s(ptr %%v) {\nentry:\n", e.aggBoxSym(a))
	sb.WriteString("  %arr = call ptr @rt_array(i64 0, ptr null)\n")
	f := *a.Arr
	e.countLoop(&sb, a.Len, func(j string) {
		b := e.boxLeafAt(&sb, f, e.elemAt(&sb, a, "%v", j))
		fmt.Fprintf(&sb, "  call void @rt_array_push(ptr %%arr, ptr %s)\n", b)
	})
	sb.WriteString("  ret ptr %arr\n}\n\n")
	return sb.String()
}

func (e *emitter) arrUnboxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal void %s(ptr %%d, ptr %%b) {\nentry:\n", e.aggUnboxSym(a))
	if f := *a.Arr; !f.Unit {
		e.countLoop(&sb, a.Len, func(j string) {
			x := e.newTmp()
			fmt.Fprintf(&sb, "  %s = call ptr @rt_at(ptr %%b, i64 %s)\n", x, j)
			e.unboxLeafAt(&sb, f, e.elemAt(&sb, a, "%d", j), x)
		})
	}
	sb.WriteString("  ret void\n}\n\n")
	return sb.String()
}
