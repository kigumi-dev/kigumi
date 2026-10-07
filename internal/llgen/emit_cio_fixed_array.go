package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/sem"
)

func (e *emitter) descSym(ent sem.EntityID) string {
	x := e.r.Entity(ent)
	return sanitize(e.r.Packages[x.Pkg].Path + "." + x.Name)
}

// The C ABI treats FixedArray[elem, N] as inline elem[N]; its boxed form is an Array of exactly N elements.
func (e *emitter) fixedArrayShape(t sem.TypeID) (elem sem.TypeID, n int64, ok bool) {
	return e.r.FixedElem(t)
}

func (e *emitter) readFixedArray(sb *strings.Builder, base string, elem sem.TypeID, n, align int64) string {
	es, _ := e.cLayoutOf(elem)
	elemAlign := min(align, es)
	ct := e.cType(elem)
	// A bare alloca, not stackSlot/argArray: this shim has no entry block
	// for stackSlot's hoisting to collect into.
	items := e.newTmp()
	fmt.Fprintf(sb, "  %s = alloca [%d x ptr]\n", items, max(n, 1))
	for i := int64(0); i < n; i++ {
		at := e.newTmp()
		fmt.Fprintf(sb, "  %s = getelementptr i8, ptr %s, i64 %d\n", at, base, i*es)
		raw := e.newTmp()
		fmt.Fprintf(sb, "  %s = load %s, ptr %s, align %d\n", raw, ct, at, elemAlign)
		v := e.box(sb, raw, elem, ct)
		slot := e.newTmp()
		fmt.Fprintf(sb, "  %s = getelementptr [%d x ptr], ptr %s, i64 0, i64 %d\n  store ptr %s, ptr %s\n", slot, max(n, 1), items, i, v, slot)
	}
	return e.irb(sb).Call(rtArray, vi64(int(n)), vptr(items)).Text
}

// writeFixedArray is the cWriter counterpart of readFixedArray.
func (e *emitter) writeFixedArray(sb *strings.Builder, base, arr string, elem sem.TypeID, n, align int64) {
	es, _ := e.cLayoutOf(elem)
	elemAlign := min(align, es)
	ct := e.cType(elem)
	for i := int64(0); i < n; i++ {
		item := e.irb(sb).Call(rtAt, vptr(arr), vi64(int(i))).Text
		raw := e.unbox(sb, item, elem, ct)
		at := e.newTmp()
		fmt.Fprintf(sb, "  %s = getelementptr i8, ptr %s, i64 %d\n", at, base, i*es)
		fmt.Fprintf(sb, "  store %s %s, ptr %s, align %d\n", ct, raw, at, elemAlign)
		e.irb(sb).Call(rtRelease, vptr(item))
	}
}
