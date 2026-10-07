package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) aggBoxSym(a *aggInfo) string   { return "@\"kg.box." + e.r.TypeString(a.T) + "\"" }
func (e *emitter) aggUnboxSym(a *aggInfo) string { return "@\"kg.unbox." + e.r.TypeString(a.T) + "\"" }

// The only two places a direct aggregate crosses into the boxed value model.
func (e *emitter) aggHelpers(a *aggInfo) {
	if e.aggDone[a.T] {
		return
	}
	e.aggDone[a.T] = true
	for _, f := range a.Fields {
		if f.Agg != nil {
			e.aggHelpers(f.Agg)
		}
	}
	e.helpers.WriteString(e.aggBoxFn(a))
	e.helpers.WriteString(e.aggUnboxFn(a))
}

func (e *emitter) aggBoxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal ptr %s(%s %%v) {\nentry:\n", e.aggBoxSym(a), a.Name)
	fmt.Fprintf(&sb, "  %%arr = alloca ptr, i32 %d\n", len(a.Fields))
	for i, f := range a.Fields {
		fmt.Fprintf(&sb, "  %%f%d = extractvalue %s %%v, %d\n", i, a.Name, i)
		if f.Agg != nil {
			fmt.Fprintf(&sb, "  %%b%d = call ptr %s(%s %%f%d)\n", i, e.aggBoxSym(f.Agg), f.Agg.Name, i)
		} else {
			boxLeaf(&sb, i, f)
		}
		fmt.Fprintf(&sb, "  %%p%d = getelementptr ptr, ptr %%arr, i32 %d\n", i, i)
		fmt.Fprintf(&sb, "  store ptr %%b%d, ptr %%p%d\n", i, i)
	}
	fmt.Fprintf(&sb, "  %%r = call ptr @rt_record(ptr %s, i64 %d, ptr %%arr)\n", e.desc(a.Ent), len(a.Fields))
	sb.WriteString("  ret ptr %r\n}\n\n")
	return sb.String()
}

// A Bool widens to the `int` rt_bool takes, as boolBox does on the scalar path.
func boxLeaf(sb *strings.Builder, i int, f aggField) {
	v := fmt.Sprintf("%%f%d", i)
	if f.Ty == TI1 {
		v = fmt.Sprintf("%%w%d", i)
		fmt.Fprintf(sb, "  %s = zext i1 %%f%d to i32\n", v, i)
	}
	fmt.Fprintf(sb, "  %%b%d = call ptr @%s\n", i, boxCallText(f, v))
}

func boxCallText(f aggField, v string) string {
	switch f.Ty {
	case TI64:
		return fmt.Sprintf("rt_int(i64 %s, i32 %d)", v, f.Nk)
	case TDouble:
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
	case TDouble:
		return fmt.Sprintf("double @rt_float_val(ptr %s)", v)
	case TI1:
		return fmt.Sprintf("i1 @rt_truth(ptr %s)", v)
	}
	return fmt.Sprintf("i32 @rt_char_val(ptr %s)", v)
}

func (e *emitter) aggUnboxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal %s %s(ptr %%b) {\nentry:\n", a.Name, e.aggUnboxSym(a))
	prev := "undef"
	for i, f := range a.Fields {
		fmt.Fprintf(&sb, "  %%f%d = call ptr @rt_field(ptr %%b, i64 %d)\n", i, i)
		if f.Agg != nil {
			fmt.Fprintf(&sb, "  %%s%d = call %s %s(ptr %%f%d)\n", i, f.Agg.Name, e.aggUnboxSym(f.Agg), i)
		} else {
			fmt.Fprintf(&sb, "  %%s%d = call %s\n", i, unboxCallText(f, fmt.Sprintf("%%f%d", i)))
		}
		fmt.Fprintf(&sb, "  call void @rt_release(ptr %%f%d)\n", i)
		fmt.Fprintf(&sb, "  %%a%d = insertvalue %s %s, %s %%s%d, %d\n", i, a.Name, prev, f.llType(), i, i)
		prev = fmt.Sprintf("%%a%d", i)
	}
	fmt.Fprintf(&sb, "  ret %s %s\n}\n\n", a.Name, prev)
	return sb.String()
}

// The caller releases the box when the instruction only borrows what it is given.
func (e *emitter) boxAgg(sb *strings.Builder, a *aggInfo, v string) string {
	e.aggHelpers(a)
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = call ptr %s(%s %s)\n", t, e.aggBoxSym(a), a.Name, v)
	return t
}

func (e *emitter) unboxAgg(sb *strings.Builder, a *aggInfo, ptrVal string) string {
	e.aggHelpers(a)
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = call %s %s(ptr %s)\n", t, a.Name, e.aggUnboxSym(a), ptrVal)
	return t
}

// vals and the result are owned ptr operands.
func (e *emitter) callBoxed(sb *strings.Builder, f *mir.Func, vals []string) string {
	s := e.directSig(f)
	if s == nil {
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = call ptr %s(%s)\n", t, e.sym(f), joinArgs(vals))
		return t
	}
	var args []string
	out := ""
	if s.Ret != nil && !byValueAgg(s.Ret) {
		out = e.stackSlot(sb, s.Ret.Name)
		args = append(args, "ptr "+out)
	}
	for i, v := range vals {
		a := s.param(i)
		if a == nil {
			args = append(args, "ptr "+v)
			continue
		}
		u := e.unboxAgg(sb, a, v)
		// A lent position never owned the box, so the reference stays where it was.
		if !e.p.ParamLent(f.Ent, i) {
			e.irb(sb).Call(rtRelease, vptr(v))
		}
		if byValueAgg(a) {
			args = append(args, a.Name+" "+u)
			continue
		}
		slot := e.stackSlot(sb, a.Name)
		fmt.Fprintf(sb, "  store %s %s, ptr %s\n", a.Name, u, slot)
		args = append(args, "ptr "+slot)
	}
	if s.Ret == nil {
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = call ptr %s(%s)\n", t, e.sym(f), strings.Join(args, ", "))
		return t
	}
	v := e.newTmp()
	if byValueAgg(s.Ret) {
		fmt.Fprintf(sb, "  %s = call %s %s(%s)\n", v, s.Ret.Name, e.sym(f), strings.Join(args, ", "))
	} else {
		fmt.Fprintf(sb, "  call void %s(%s)\n", e.sym(f), strings.Join(args, ", "))
		fmt.Fprintf(sb, "  %s = load %s, ptr %s\n", v, s.Ret.Name, out)
	}
	return e.boxAgg(sb, s.Ret, v)
}
