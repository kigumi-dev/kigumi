package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/mir"
)

const memIntrinsics = "declare void @llvm.memcpy.p0.p0.i64(ptr, ptr, i64, i1)\ndeclare void @llvm.memset.p0.i64(ptr, i8, i64, i1)\n\n"

// The slot is two i64 words, which no supported target aligns below 4.
const wideAccessAlign = 4

func aggPtr(l mir.LocalID) string { return fmt.Sprintf("%%l%d", l) }

// LLVM's own size for the type, folded to a constant, never a second layout to keep in step.
func sizeOf(a *aggInfo) string {
	return fmt.Sprintf("ptrtoint (ptr getelementptr (%s, ptr null, i32 1) to i64)", a.Name)
}

// LLVM splits a first-class aggregate load or store into one operation per element.
func copyAgg(sb *strings.Builder, a *aggInfo, dst, src string) {
	fmt.Fprintf(sb, "  call void @llvm.memcpy.p0.p0.i64(ptr %s, ptr %s, i64 %s, i1 false)\n", dst, src, sizeOf(a))
}

func zeroAgg(sb *strings.Builder, a *aggInfo, dst string) {
	fmt.Fprintf(sb, "  call void @llvm.memset.p0.i64(ptr %s, i8 0, i64 %s, i1 false)\n", dst, sizeOf(a))
}

func (e *emitter) fieldPtr(sb *strings.Builder, a *aggInfo, base string, i int) string {
	g := e.newTmp()
	fmt.Fprintf(sb, "  %s = getelementptr %s, ptr %s, i32 0, i32 %d\n", g, a.Name, base, i)
	return g
}

func (e *emitter) storeLeaf(sb *strings.Builder, fld aggField, p string, l mir.LocalID) {
	switch {
	case fld.Unit:
	case fld.Agg != nil:
		copyAgg(sb, fld.Agg, p, aggPtr(l))
	case fld.Ref:
		e.storeRef(sb, p, e.load(sb, l))
	default:
		e.storeLeafScalar(sb, fld, p, e.leafScalar(sb, fld, l))
	}
}

func (e *emitter) loadLeaf(sb *strings.Builder, f *mir.Func, fld aggField, p string, dst mir.LocalID) {
	switch {
	case fld.Unit:
		e.store(sb, dst, e.irb(sb).Call(rtUnit).Text)
	case fld.Agg != nil:
		copyAgg(sb, fld.Agg, aggPtr(dst), p)
		if fld.Agg.counted() {
			e.refCall(sb, fld.Agg, refRetain, aggPtr(dst))
		}
	case fld.Ref:
		e.store(sb, dst, e.irb(sb).Call(rtRetain, vptr(e.loadRef(sb, p))).Text)
	default:
		e.storeScalarResult(sb, f, dst, Value{fld.Ty, e.loadLeafScalar(sb, fld, p)})
	}
}

func (e *emitter) leafScalar(sb *strings.Builder, fld aggField, l mir.LocalID) string {
	if _, ok := e.scalar[l]; ok {
		return e.loadNative(sb, l).Text
	}
	p := e.load(sb, l)
	v := e.unboxScalar(sb, fld.Ty, p)
	e.irb(sb).Call(rtRelease, vptr(p))
	return v.Text
}

// A narrow array element is stored truncated and read back extended by its signedness.
func (e *emitter) storeLeafScalar(sb *strings.Builder, fld aggField, p, v string) {
	m := fld.mem()
	if m == TI128 {
		fmt.Fprintf(sb, "  store i128 %s, ptr %s, align %d\n", v, p, wideAccessAlign)
		return
	}
	if m != fld.Ty {
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = trunc %s %s to %s\n", t, fld.Ty, v, m)
		v = t
	}
	fmt.Fprintf(sb, "  store %s %s, ptr %s\n", m, v, p)
}

func (e *emitter) loadLeafScalar(sb *strings.Builder, fld aggField, p string) string {
	m := fld.mem()
	v := e.newTmp()
	if m == TI128 {
		fmt.Fprintf(sb, "  %s = load i128, ptr %s, align %d\n", v, p, wideAccessAlign)
		return v
	}
	fmt.Fprintf(sb, "  %s = load %s, ptr %s\n", v, m, p)
	if m == fld.Ty {
		return v
	}
	ext := "zext"
	if fld.Nk&(1<<8) != 0 {
		ext = "sext"
	}
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = %s %s %s to %s\n", t, ext, m, v, fld.Ty)
	return t
}

// Only the by-value ABI reads a whole aggregate as a first-class operand.
func (e *emitter) aggValue(sb *strings.Builder, a *aggInfo, p string) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = load %s, ptr %s\n", t, a.Name, p)
	return t
}
