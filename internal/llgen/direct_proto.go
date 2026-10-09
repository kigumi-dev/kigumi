package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

type protoKey struct {
	agg  *aggInfo
	hash bool
}

func (e *emitter) protoSym(a *aggInfo, hash bool) string {
	if hash {
		return "@\"kg.hash." + a.Label + "\""
	}
	return "@\"kg.eq." + a.Label + "\""
}

func (e *emitter) protoBuiltin(sb *strings.Builder, f *mir.Func, in mir.Inst) {
	a, ok := e.direct[in.Args[0]]
	if !ok {
		panic("llgen: " + in.Str + " over a boxed operand")
	}
	hash := in.Str == "direct.hash"
	e.protoHelper(a, hash)
	v := e.newTmp()
	if hash {
		fmt.Fprintf(sb, "  %s = call i64 %s(ptr %s)\n", v, e.protoSym(a, true), aggPtr(in.Args[0]))
		e.storeScalarResult(sb, f, in.Dst, Value{TI64, v})
		return
	}
	fmt.Fprintf(sb, "  %s = call i1 %s(ptr %s, ptr %s)\n", v, e.protoSym(a, false), aggPtr(in.Args[0]), aggPtr(in.Args[1]))
	e.storeScalarResult(sb, f, in.Dst, Value{TI1, v})
}

func (e *emitter) protoHelper(a *aggInfo, hash bool) {
	key := protoKey{a, hash}
	if e.protoDone == nil {
		e.protoDone = map[protoKey]bool{}
	}
	if e.protoDone[key] {
		return
	}
	e.protoDone[key] = true
	for _, f := range a.leaves() {
		if f.Agg != nil {
			e.protoHelper(f.Agg, hash)
		}
	}
	var sb strings.Builder
	if hash {
		fmt.Fprintf(&sb, "define internal i64 %s(ptr %%a) {\nentry:\n", e.protoSym(a, true))
	} else {
		fmt.Fprintf(&sb, "define internal i1 %s(ptr %%a, ptr %%b) {\nentry:\n", e.protoSym(a, false))
	}
	switch {
	case a.isAdt():
		e.protoAdt(&sb, a, hash)
	case hash:
		e.protoHashFields(&sb, offsetBasis, a.Fields, func(i int) string { return e.fieldPtr(&sb, a, "%a", i) })
	default:
		e.protoEqFields(&sb, a.Fields, func(side string, i int) string { return e.fieldPtr(&sb, a, side, i) })
	}
	sb.WriteString("}\n\n")
	e.helpers.WriteString(sb.String())
}

// 14695981039346656037 as a signed 64-bit literal.
const offsetBasis = "-3750763034362895579"
const fnvPrimeText = "1099511628211"

func (e *emitter) protoEqFields(sb *strings.Builder, fs []aggField, at func(side string, i int) string) {
	all := "true"
	for i, f := range fs {
		c := e.leafEq(sb, f, at("%a", i), at("%b", i))
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = and i1 %s, %s\n", t, all, c)
		all = t
	}
	fmt.Fprintf(sb, "  ret i1 %s\n", all)
}

func (e *emitter) protoHashFields(sb *strings.Builder, start string, fs []aggField, at func(i int) string) {
	h := start
	for i, f := range fs {
		x, m := e.newTmp(), e.newTmp()
		fmt.Fprintf(sb, "  %s = xor i64 %s, %s\n  %s = mul i64 %s, %s\n", x, h, e.leafHash(sb, f, at(i)), m, x, fnvPrimeText)
		h = m
	}
	fmt.Fprintf(sb, "  ret i64 %s\n", h)
}

func (e *emitter) protoAdt(sb *strings.Builder, a *aggInfo, hash bool) {
	tag := e.newTmp()
	fmt.Fprintf(sb, "  %s = load %s, ptr %s\n", tag, adtTag, e.tagPtr(sb, a, "%a"))
	if !hash {
		other, same := e.newTmp(), e.newTmp()
		fmt.Fprintf(sb, "  %s = load %s, ptr %s\n  %s = icmp eq %s %s, %s\n  br i1 %s, label %%cmp, label %%no\ncmp:\n", other, adtTag, e.tagPtr(sb, a, "%b"), same, adtTag, tag, other, same)
	}
	fmt.Fprintf(sb, "  switch %s %s, label %%no [\n", adtTag, tag)
	for i := range a.Variants {
		fmt.Fprintf(sb, "    %s %d, label %%arm%d\n", adtTag, i, i)
	}
	sb.WriteString("  ]\n")
	for i, v := range a.Variants {
		fmt.Fprintf(sb, "arm%d:\n", i)
		slot := func(side string, j int) string { return e.slotPtr(sb, a, side, j) }
		switch {
		case hash:
			e.protoHashFields(sb, fmt.Sprint(i+1), v.Payload, func(j int) string { return slot("%a", j) })
		default:
			e.protoEqFields(sb, v.Payload, slot)
		}
	}
	if hash {
		sb.WriteString("no:\n  unreachable\n")
		return
	}
	sb.WriteString("no:\n  ret i1 false\n")
}

func (e *emitter) leafEq(sb *strings.Builder, f aggField, pa, pb string) string {
	r := e.newTmp()
	if f.Agg != nil {
		fmt.Fprintf(sb, "  %s = call i1 %s(ptr %s, ptr %s)\n", r, e.protoSym(f.Agg, false), pa, pb)
		return r
	}
	x, y := e.loadLeafScalar(sb, f, pa), e.loadLeafScalar(sb, f, pb)
	if f.Ty != TDouble {
		fmt.Fprintf(sb, "  %s = icmp eq %s %s, %s\n", r, f.Ty, x, y)
		return r
	}
	// The Eq protocol treats every NaN as equal and -0.0 as 0.0, unlike IEEE `==`.
	ordered, xn, yn, both := e.newTmp(), e.newTmp(), e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = fcmp oeq double %s, %s\n  %s = fcmp uno double %s, %s\n  %s = fcmp uno double %s, %s\n  %s = and i1 %s, %s\n  %s = or i1 %s, %s\n",
		ordered, x, y, xn, x, x, yn, y, y, both, xn, yn, r, ordered, both)
	return r
}

func (e *emitter) leafHash(sb *strings.Builder, f aggField, p string) string {
	if f.Agg != nil {
		r := e.newTmp()
		fmt.Fprintf(sb, "  %s = call i64 %s(ptr %s)\n", r, e.protoSym(f.Agg, true), p)
		return r
	}
	v, b := e.loadLeafScalar(sb, f, p), e.irb(sb)
	switch f.Ty {
	case TI64:
		return b.Call(rtDHashInt, Value{TI64, v}, vi32(f.Nk)).Text
	case TI1:
		w := e.newTmp()
		fmt.Fprintf(sb, "  %s = zext i1 %s to i32\n", w, v)
		return b.Call(rtDHashBool, vi32text(w)).Text
	case TDouble:
		return b.Call(rtDHashFloat, vdouble(v), vi32(f.Nk)).Text
	}
	return b.Call(rtDHashChar, Value{TI32, v}).Text
}
