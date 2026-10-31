package llgen

import (
	"fmt"
	"strings"
)

// The tag and payload area are private to this module; these helpers are the whole contract with the boxed form.
func (e *emitter) adtPrelude(sb *strings.Builder, a *aggInfo, base string) {
	fmt.Fprintf(sb, "  %%tp = getelementptr %s, ptr %s, i32 0, i32 0\n", a.Name, base)
	fmt.Fprintf(sb, "  %%pb = getelementptr %s, ptr %s, i32 0, i32 1\n", a.Name, base)
}

func (e *emitter) adtSwitch(sb *strings.Builder, a *aggInfo) {
	fmt.Fprintf(sb, "  switch %s %%tag, label %%bad [", adtTag)
	for i := range a.Variants {
		fmt.Fprintf(sb, " %s %d, label %%c%d", adtTag, i, i)
	}
	sb.WriteString(" ]\n")
}

func (e *emitter) adtBoxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal ptr %s(ptr %%v) {\nentry:\n", e.aggBoxSym(a))
	e.adtPrelude(&sb, a, "%v")
	fmt.Fprintf(&sb, "  %%arr = alloca ptr, i32 %d\n", maxI64(int64(len(a.Slots)), 1))
	fmt.Fprintf(&sb, "  %%tag = load %s, ptr %%tp\n", adtTag)
	e.adtSwitch(&sb, a)
	for i, v := range a.Variants {
		fmt.Fprintf(&sb, "c%d:\n", i)
		for j, f := range v.Payload {
			b := e.boxLeafAt(&sb, f, e.gepSlot(&sb, j, a))
			p := e.newTmp()
			fmt.Fprintf(&sb, "  %s = getelementptr ptr, ptr %%arr, i32 %d\n  store ptr %s, ptr %s\n", p, j, b, p)
		}
		r := e.newTmp()
		fmt.Fprintf(&sb, "  %s = call ptr @rt_variant(ptr %s, i64 %d, ptr %%arr)\n", r, e.vdesc(v.Ent), len(v.Payload))
		fmt.Fprintf(&sb, "  ret ptr %s\n", r)
	}
	sb.WriteString("bad:\n  unreachable\n}\n\n")
	return sb.String()
}

func (e *emitter) gepSlot(sb *strings.Builder, j int, a *aggInfo) string {
	if a.Slots[j] == 0 {
		return "%pb"
	}
	g := e.newTmp()
	fmt.Fprintf(sb, "  %s = getelementptr i8, ptr %%pb, i64 %d\n", g, a.Slots[j])
	return g
}

func (e *emitter) adtUnboxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal void %s(ptr %%d, ptr %%b) {\nentry:\n", e.aggUnboxSym(a))
	e.adtPrelude(&sb, a, "%d")
	zeroAgg(&sb, a, "%d")
	fmt.Fprintf(&sb, "  %%tag = call %s @rt_variant_index(ptr %%b)\n", adtTag)
	fmt.Fprintf(&sb, "  store %s %%tag, ptr %%tp\n", adtTag)
	e.adtSwitch(&sb, a)
	for i, v := range a.Variants {
		fmt.Fprintf(&sb, "c%d:\n", i)
		for j, f := range v.Payload {
			if f.Unit {
				continue
			}
			q := e.newTmp()
			fmt.Fprintf(&sb, "  %s = call ptr @rt_payload(ptr %%b, i64 %d)\n", q, j)
			e.unboxLeafAt(&sb, f, e.gepSlot(&sb, j, a), q)
		}
		sb.WriteString("  br label %done\n")
	}
	sb.WriteString("bad:\n  unreachable\ndone:\n  ret void\n}\n\n")
	return sb.String()
}
