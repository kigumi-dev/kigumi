package llgen

import (
	"fmt"
	"strings"
)

func (e *emitter) aggBoxSym(a *aggInfo) string   { return "@\"kg.box." + a.Label + "\"" }
func (e *emitter) aggUnboxSym(a *aggInfo) string { return "@\"kg.unbox." + a.Label + "\"" }

func (e *emitter) aggHelpers(a *aggInfo) {
	if e.aggDone[a.T] {
		return
	}
	e.aggDone[a.T] = true
	for _, f := range a.leaves() {
		if f.Agg != nil {
			e.aggHelpers(f.Agg)
		}
	}
	switch {
	case a.isAdt():
		e.helpers.WriteString(e.adtBoxFn(a))
		e.helpers.WriteString(e.adtUnboxFn(a))
	case a.Clo != nil:
		e.helpers.WriteString(e.cloBoxFn(a))
		e.helpers.WriteString(e.cloUnboxFn(a))
	case a.Arr != nil:
		e.helpers.WriteString(e.arrBoxFn(a))
		e.helpers.WriteString(e.arrUnboxFn(a))
	default:
		e.helpers.WriteString(e.aggBoxFn(a))
		e.helpers.WriteString(e.aggUnboxFn(a))
	}
}

func (a *aggInfo) leaves() []aggField {
	switch {
	case a.Arr != nil:
		return []aggField{*a.Arr}
	case !a.isAdt():
		return a.Fields
	}
	var out []aggField
	for _, v := range a.Variants {
		out = append(out, v.Payload...)
	}
	return out
}

func (e *emitter) aggBoxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal ptr %s(ptr %%v) {\nentry:\n", e.aggBoxSym(a))
	arr, _ := e.boxLeaves(&sb, a, "%v")
	fmt.Fprintf(&sb, "  %%r = call ptr @rt_record(ptr %s, i64 %d, ptr %s)\n", e.desc(a.Ent), len(a.Fields), arr)
	sb.WriteString("  ret ptr %r\n}\n\n")
	return sb.String()
}

func (e *emitter) boxLeaves(sb *strings.Builder, a *aggInfo, src string) (arr string, boxes []string) {
	arr = e.newTmp()
	fmt.Fprintf(sb, "  %s = alloca ptr, i32 %d\n", arr, max(len(a.Fields), 1))
	for i, f := range a.Fields {
		b := e.boxLeafAt(sb, f, e.fieldPtr(sb, a, src, i))
		p := e.newTmp()
		fmt.Fprintf(sb, "  %s = getelementptr ptr, ptr %s, i32 %d\n  store ptr %s, ptr %s\n", p, arr, i, b, p)
		boxes = append(boxes, b)
	}
	return arr, boxes
}

func (e *emitter) boxLeafAt(sb *strings.Builder, f aggField, p string) string {
	if f.Ref {
		return e.loadRef(sb, p)
	}
	b := e.newTmp()
	switch {
	case f.Unit:
		fmt.Fprintf(sb, "  %s = call ptr @rt_unit()\n", b)
	case f.Agg != nil:
		fmt.Fprintf(sb, "  %s = call ptr %s(ptr %s)\n", b, e.aggBoxSym(f.Agg), p)
	case f.Ty == TI128:
		b = e.boxNative(sb, scalarInfo{Ty: TI128, Nk: f.Nk}, Value{TI128, e.loadLeafScalar(sb, f, p)})
	default:
		v := e.loadLeafScalar(sb, f, p)
		// rt_bool takes an int, so a Bool widens.
		if f.Ty == TI1 {
			w := e.newTmp()
			fmt.Fprintf(sb, "  %s = zext i1 %s to i32\n", w, v)
			v = w
		}
		if f.Ty == TFloat {
			v = e.extendFloat(sb, v)
		}
		fmt.Fprintf(sb, "  %s = call ptr @%s\n", b, boxCallText(f, v))
	}
	return b
}

func (e *emitter) unboxLeafAt(sb *strings.Builder, f aggField, p, box string) {
	if f.Ref {
		e.storeRef(sb, p, box)
		return
	}
	switch {
	case f.Unit:
	case f.Agg != nil:
		fmt.Fprintf(sb, "  call void %s(ptr %s, ptr %s)\n", e.aggUnboxSym(f.Agg), p, box)
	case f.Ty == TI128:
		e.storeLeafScalar(sb, f, p, e.unboxScalar(sb, TI128, box).Text)
	default:
		v := e.newTmp()
		fmt.Fprintf(sb, "  %s = call %s\n", v, unboxCallText(f, box))
		if f.Ty == TFloat {
			v = e.narrowFloat(sb, v)
		}
		e.storeLeafScalar(sb, f, p, v)
	}
	fmt.Fprintf(sb, "  call void @rt_release(ptr %s)\n", box)
}

func boxCallText(f aggField, v string) string {
	switch f.Ty {
	case TI64:
		return fmt.Sprintf("rt_int(i64 %s, i32 %d)", v, f.Nk)
	case TDouble, TFloat:
		return fmt.Sprintf("rt_float(double %s, i32 %d)", v, f.Nk)
	case TI1:
		return fmt.Sprintf("rt_bool(i32 %s)", v)
	}
	return fmt.Sprintf("rt_char(i32 %s)", v)
}

func unboxCallText(f aggField, v string) string {
	switch f.Ty {
	case TI64:
		return fmt.Sprintf("i64 @rt_int_val(ptr %s)", v)
	case TDouble, TFloat:
		return fmt.Sprintf("double @rt_float_val(ptr %s)", v)
	case TI1:
		return fmt.Sprintf("i1 @rt_truth(ptr %s)", v)
	}
	return fmt.Sprintf("i32 @rt_char_val(ptr %s)", v)
}

func (e *emitter) aggUnboxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal void %s(ptr %%d, ptr %%b) {\nentry:\n", e.aggUnboxSym(a))
	e.unboxLeaves(&sb, a, "%d", func(i int) string {
		return fmt.Sprintf("call ptr @rt_field(ptr %%b, i64 %d)", i)
	})
	sb.WriteString("  ret void\n}\n\n")
	return sb.String()
}

func (e *emitter) unboxLeaves(sb *strings.Builder, a *aggInfo, dst string, fetch func(int) string) {
	for i, f := range a.Fields {
		if f.Unit {
			continue
		}
		box := e.newTmp()
		fmt.Fprintf(sb, "  %s = %s\n", box, fetch(i))
		e.unboxLeafAt(sb, f, e.fieldPtr(sb, a, dst, i), box)
	}
}

// The caller releases the box when the instruction only borrows what it is given.
func (e *emitter) boxAgg(sb *strings.Builder, a *aggInfo, src string) string {
	e.aggHelpers(a)
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = call ptr %s(ptr %s)\n", t, e.aggBoxSym(a), src)
	return t
}

func (e *emitter) unboxAgg(sb *strings.Builder, a *aggInfo, dst, box string) {
	e.aggHelpers(a)
	fmt.Fprintf(sb, "  call void %s(ptr %s, ptr %s)\n", e.aggUnboxSym(a), dst, box)
}
