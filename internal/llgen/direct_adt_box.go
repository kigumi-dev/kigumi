package llgen

import (
	"fmt"
	"strings"
)

// The tag and payload area are private to this module; these helpers are the whole contract with the boxed form.
func (e *emitter) adtPrelude(sb *strings.Builder, a *aggInfo) {
	fmt.Fprintf(sb, "  %%slot = alloca %s\n", a.Name)
	fmt.Fprintf(sb, "  %%tp = getelementptr %s, ptr %%slot, i32 0, i32 0\n", a.Name)
	fmt.Fprintf(sb, "  %%pb = getelementptr %s, ptr %%slot, i32 0, i32 1\n", a.Name)
}

func (e *emitter) adtSwitch(sb *strings.Builder, a *aggInfo) {
	fmt.Fprintf(sb, "  switch %s %%tag, label %%bad [", adtTag)
	for i := range a.Variants {
		fmt.Fprintf(sb, " %s %d, label %%c%d", adtTag, i, i)
	}
	sb.WriteString(" ]\n")
}

func slotName(prefix string, i, j int) string { return fmt.Sprintf("%%%s%d_%d", prefix, i, j) }

func (e *emitter) adtBoxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal ptr %s(%s %%v) {\nentry:\n", e.aggBoxSym(a), a.Name)
	e.adtPrelude(&sb, a)
	fmt.Fprintf(&sb, "  store %s %%v, ptr %%slot\n", a.Name)
	fmt.Fprintf(&sb, "  %%arr = alloca ptr, i32 %d\n", maxI64(int64(len(a.Slots)), 1))
	fmt.Fprintf(&sb, "  %%tag = load %s, ptr %%tp\n", adtTag)
	e.adtSwitch(&sb, a)
	for i, v := range a.Variants {
		fmt.Fprintf(&sb, "c%d:\n", i)
		for j, f := range v.Payload {
			e.boxSlot(&sb, a, i, j, f)
		}
		r := fmt.Sprintf("%%r%d", i)
		fmt.Fprintf(&sb, "  %s = call ptr @rt_variant(ptr %s, i64 %d, ptr %%arr)\n", r, e.vdesc(v.Ent), len(v.Payload))
		fmt.Fprintf(&sb, "  ret ptr %s\n", r)
	}
	sb.WriteString("bad:\n  unreachable\n}\n\n")
	return sb.String()
}

func (e *emitter) boxSlot(sb *strings.Builder, a *aggInfo, i, j int, f aggField) {
	box := slotName("b", i, j)
	switch {
	case f.Unit:
		fmt.Fprintf(sb, "  %s = call ptr @rt_unit()\n", box)
	case f.Agg != nil:
		v := e.loadSlot(sb, a, i, j, f)
		fmt.Fprintf(sb, "  %s = call ptr %s(%s %s)\n", box, e.aggBoxSym(f.Agg), f.Agg.Name, v)
	default:
		v := e.loadSlot(sb, a, i, j, f)
		if f.Ty == TI1 {
			w := slotName("w", i, j)
			fmt.Fprintf(sb, "  %s = zext i1 %s to i32\n", w, v)
			v = w
		}
		fmt.Fprintf(sb, "  %s = call ptr @%s\n", box, boxCallText(f, v))
	}
	p := slotName("p", i, j)
	fmt.Fprintf(sb, "  %s = getelementptr ptr, ptr %%arr, i32 %d\n", p, j)
	fmt.Fprintf(sb, "  store ptr %s, ptr %s\n", box, p)
}

func (e *emitter) loadSlot(sb *strings.Builder, a *aggInfo, i, j int, f aggField) string {
	g := e.gepSlot(sb, a, i, j)
	v := slotName("f", i, j)
	fmt.Fprintf(sb, "  %s = load %s, ptr %s\n", v, f.llType(), g)
	return v
}

func (e *emitter) gepSlot(sb *strings.Builder, a *aggInfo, i, j int) string {
	if a.Slots[j] == 0 {
		return "%pb"
	}
	g := slotName("g", i, j)
	fmt.Fprintf(sb, "  %s = getelementptr i8, ptr %%pb, i64 %d\n", g, a.Slots[j])
	return g
}

func (e *emitter) adtUnboxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal %s %s(ptr %%b) {\nentry:\n", a.Name, e.aggUnboxSym(a))
	e.adtPrelude(&sb, a)
	fmt.Fprintf(&sb, "  store %s zeroinitializer, ptr %%slot\n", a.Name)
	fmt.Fprintf(&sb, "  %%tag = call %s @rt_variant_index(ptr %%b)\n", adtTag)
	fmt.Fprintf(&sb, "  store %s %%tag, ptr %%tp\n", adtTag)
	e.adtSwitch(&sb, a)
	for i, v := range a.Variants {
		fmt.Fprintf(&sb, "c%d:\n", i)
		for j, f := range v.Payload {
			e.unboxSlot(&sb, a, i, j, f)
		}
		sb.WriteString("  br label %done\n")
	}
	sb.WriteString("bad:\n  unreachable\ndone:\n")
	fmt.Fprintf(&sb, "  %%r = load %s, ptr %%slot\n  ret %s %%r\n}\n\n", a.Name, a.Name)
	return sb.String()
}

func (e *emitter) unboxSlot(sb *strings.Builder, a *aggInfo, i, j int, f aggField) {
	if f.Unit {
		return
	}
	p := slotName("q", i, j)
	fmt.Fprintf(sb, "  %s = call ptr @rt_payload(ptr %%b, i64 %d)\n", p, j)
	v := slotName("s", i, j)
	if f.Agg != nil {
		fmt.Fprintf(sb, "  %s = call %s %s(ptr %s)\n", v, f.Agg.Name, e.aggUnboxSym(f.Agg), p)
	} else {
		fmt.Fprintf(sb, "  %s = call %s\n", v, unboxCallText(f, p))
	}
	fmt.Fprintf(sb, "  call void @rt_release(ptr %s)\n", p)
	fmt.Fprintf(sb, "  store %s %s, ptr %s\n", f.llType(), v, e.gepSlot(sb, a, i, j))
}
