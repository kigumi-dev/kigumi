package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// simdLowered maps a std/simd key (without the std/ prefix) to the operation
// llgen expands at the call site. None of them has a runtime entry.
var simdLowered = map[string]string{
	"simd.Vec.splat":     "splat",
	"simd.Vec.fromArray": "fromArray",
	"simd.Vec.toArray":   "toArray",
	"simd.Vec.add":       "add",
	"simd.Vec.sub":       "sub",
	"simd.Vec.mul":       "mul",
}

// simdArchVector reports whether arch has a 128-bit integer and float vector
// register every lane type maps onto. Both x86-64 (SSE2) and AArch64 (NEON)
// guarantee it as part of the base architecture, so no target feature is
// needed; every other target runs the scalar lane loop.
func simdArchVector(arch string) bool { return arch == "amd64" || arch == "arm64" }

// trySimdCall expands one std/simd primitive. The plan keeps every vector
// operand and result unboxed, so the lanes are the aggregate's own memory.
func (e *emitter) trySimdCall(sb *strings.Builder, f *midir.Func, in midir.Inst) bool {
	if in.Op != midir.OpCall {
		return false
	}
	op, ok := simdLowered[opdef.StdKey(e.r, in.Ent, true)]
	if !ok {
		return false
	}
	elem, n, ok := e.simdShape(f.Locals[in.Dst].Type)
	if !ok {
		panic("llgen: " + op + " has no vector shape")
	}
	lane, ok := e.simdElemType(elem)
	if !ok {
		panic("llgen: " + op + " has an unsupported lane type")
	}
	if simdArchVector(e.layout.Arch) {
		e.simdVectorCall(sb, f, in, op, lane, n, elem)
		return true
	}
	e.simdLaneCall(sb, f, in, op, lane, n, elem)
	return true
}

// simdShape reads the lane element and count of a Vec or FixedArray type.
func (e *emitter) simdShape(t sem.TypeID) (elem sem.TypeID, n int64, ok bool) {
	if elem, n, ok := e.r.SimdVecShape(t); ok {
		return elem, n, true
	}
	return e.r.FixedElem(t)
}

// simdElemType is the LLVM scalar type of one lane.
func (e *emitter) simdElemType(t sem.TypeID) (Type, bool) {
	switch t {
	case sem.TyI8, sem.TyU8:
		return TI8, true
	case sem.TyI16, sem.TyU16:
		return TI16, true
	case sem.TyI32, sem.TyU32:
		return TI32, true
	case sem.TyI64, sem.TyU64:
		return TI64, true
	case sem.TyF32:
		return TFloat, true
	case sem.TyF64:
		return TDouble, true
	}
	return TVoid, false
}

// simdLanes gives the pointer to a value's lanes and the FixedArray layout
// describing them, for either a Vec (one FixedArray field) or a FixedArray.
func (e *emitter) simdLanes(sb *strings.Builder, l midir.LocalID) (string, *aggInfo, bool) {
	a, ok := e.direct[l]
	if !ok {
		return "", nil, false
	}
	if a.Arr != nil {
		return aggPtr(l), a, true
	}
	if len(a.Fields) == 1 && a.Fields[0].Agg != nil && a.Fields[0].Agg.Arr != nil {
		return e.fieldPtr(sb, a, aggPtr(l), 0), a.Fields[0].Agg, true
	}
	return "", nil, false
}

// simdLaneAt points at lane i of a FixedArray aggregate.
func (e *emitter) simdLaneAt(sb *strings.Builder, lanes *aggInfo, base string, i int64) string {
	p := e.newTmp()
	fmt.Fprintf(sb, "  %s = getelementptr %s, ptr %s, i32 0, i32 0, i64 %d\n", p, lanes.Name, base, i)
	return p
}

func simdVectorType(lane Type, n int64) string {
	return fmt.Sprintf("<%d x %s>", n, lane.String())
}

func (e *emitter) simdBinInstr(sb *strings.Builder, op string, lane Type, vt, l, r string) string {
	mnemonic := op
	if lane == TFloat || lane == TDouble {
		mnemonic = "f" + op
	}
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = %s %s %s, %s\n", t, mnemonic, vt, l, r)
	return t
}

func (e *emitter) simdLoad(sb *strings.Builder, vt, p string) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = load %s, ptr %s, align 1\n", t, vt, p)
	return t
}

func (e *emitter) simdStore(sb *strings.Builder, vt, v, p string) {
	fmt.Fprintf(sb, "  store %s %s, ptr %s, align 1\n", vt, v, p)
}

// simdVectorCall lowers the operation to one vector instruction on a target
// with a 128-bit register.
func (e *emitter) simdVectorCall(sb *strings.Builder, f *midir.Func, in midir.Inst, op string, lane Type, n int64, elem sem.TypeID) {
	vt := simdVectorType(lane, n)
	dst, _, ok := e.simdLanes(sb, in.Dst)
	if !ok {
		panic("llgen: simd result is boxed")
	}
	switch op {
	case "splat":
		v, _ := e.scalarOperand(sb, f, in.Args[0])
		if lane != TFloat && lane != TDouble {
			v = Value{Type: lane, Text: e.narrowInt(sb, e.numKind(elem), v.Text)}
		}
		cur := "undef"
		for i := int64(0); i < n; i++ {
			t := e.newTmp()
			fmt.Fprintf(sb, "  %s = insertelement %s %s, %s %s, i32 %d\n", t, vt, cur, lane.String(), v.Text, i)
			cur = t
		}
		e.simdStore(sb, vt, cur, dst)
	case "fromArray":
		src, _, ok := e.simdLanes(sb, in.Args[0])
		if !ok {
			panic("llgen: simd source is boxed")
		}
		e.simdStore(sb, vt, e.simdLoad(sb, vt, src), dst)
	case "toArray":
		src, _, ok := e.simdLanes(sb, in.Args[0])
		if !ok {
			panic("llgen: simd source is boxed")
		}
		e.simdStore(sb, vt, e.simdLoad(sb, vt, src), dst)
	case "add", "sub", "mul":
		l, _, ok := e.simdLanes(sb, in.Args[0])
		if !ok {
			panic("llgen: simd operand is boxed")
		}
		r, _, ok := e.simdLanes(sb, in.Args[1])
		if !ok {
			panic("llgen: simd operand is boxed")
		}
		lv, rv := e.simdLoad(sb, vt, l), e.simdLoad(sb, vt, r)
		e.simdStore(sb, vt, e.simdBinInstr(sb, op, lane, vt, lv, rv), dst)
	}
}

// simdLaneCall is the scalar fallback on a target without 128-bit vectors:
// the same lane operations, one element at a time.
func (e *emitter) simdLaneCall(sb *strings.Builder, f *midir.Func, in midir.Inst, op string, lane Type, n int64, elem sem.TypeID) {
	dst, dstLanes, ok := e.simdLanes(sb, in.Dst)
	if !ok {
		panic("llgen: simd result is boxed")
	}
	switch op {
	case "splat":
		v, _ := e.scalarOperand(sb, f, in.Args[0])
		if lane != TFloat && lane != TDouble {
			v = Value{Type: lane, Text: e.narrowInt(sb, e.numKind(elem), v.Text)}
		}
		for i := int64(0); i < n; i++ {
			fmt.Fprintf(sb, "  store %s %s, ptr %s, align 1\n", lane.String(), v.Text, e.simdLaneAt(sb, dstLanes, dst, i))
		}
	case "fromArray":
		src, srcLanes, ok := e.simdLanes(sb, in.Args[0])
		if !ok {
			panic("llgen: simd source is boxed")
		}
		for i := int64(0); i < n; i++ {
			x := e.newTmp()
			fmt.Fprintf(sb, "  %s = load %s, ptr %s, align 1\n", x, lane.String(), e.simdLaneAt(sb, srcLanes, src, i))
			fmt.Fprintf(sb, "  store %s %s, ptr %s, align 1\n", lane.String(), x, e.simdLaneAt(sb, dstLanes, dst, i))
		}
	case "toArray":
		src, srcLanes, ok := e.simdLanes(sb, in.Args[0])
		if !ok {
			panic("llgen: simd source is boxed")
		}
		for i := int64(0); i < n; i++ {
			x := e.newTmp()
			fmt.Fprintf(sb, "  %s = load %s, ptr %s, align 1\n", x, lane.String(), e.simdLaneAt(sb, srcLanes, src, i))
			fmt.Fprintf(sb, "  store %s %s, ptr %s, align 1\n", lane.String(), x, e.simdLaneAt(sb, dstLanes, dst, i))
		}
	case "add", "sub", "mul":
		l, lLanes, ok := e.simdLanes(sb, in.Args[0])
		if !ok {
			panic("llgen: simd operand is boxed")
		}
		r, rLanes, ok := e.simdLanes(sb, in.Args[1])
		if !ok {
			panic("llgen: simd operand is boxed")
		}
		mnemonic := op
		if lane == TFloat || lane == TDouble {
			mnemonic = "f" + op
		}
		for i := int64(0); i < n; i++ {
			lv := e.newTmp()
			rv := e.newTmp()
			res := e.newTmp()
			fmt.Fprintf(sb, "  %s = load %s, ptr %s, align 1\n", lv, lane.String(), e.simdLaneAt(sb, lLanes, l, i))
			fmt.Fprintf(sb, "  %s = load %s, ptr %s, align 1\n", rv, lane.String(), e.simdLaneAt(sb, rLanes, r, i))
			fmt.Fprintf(sb, "  %s = %s %s %s, %s\n", res, mnemonic, lane.String(), lv, rv)
			fmt.Fprintf(sb, "  store %s %s, ptr %s, align 1\n", lane.String(), res, e.simdLaneAt(sb, dstLanes, dst, i))
		}
	}
}
