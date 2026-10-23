package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/sem"
)

// cScalarType adds i8 for Bool and i32 for Char, which extern(C) signatures
// cannot carry (E924) but CValue may.
func (e *emitter) cScalarType(t sem.TypeID) string {
	switch t {
	case sem.TyBool:
		return "i8"
	case sem.TyChar:
		return "i32"
	default:
		return e.cType(t)
	}
}

// storeScalar extends unbox's cases to Bool and Char.
func (e *emitter) storeScalar(sb *strings.Builder, v string, t sem.TypeID, ptr string) {
	e.storeScalarQ(sb, v, t, ptr, "")
}

func (e *emitter) storeScalarQ(sb *strings.Builder, v string, t sem.TypeID, ptr, q string) {
	switch t {
	case sem.TyBool:
		b := e.irb(sb).Call(rtTruth, vptr(v)).Text
		w := e.newTmp()
		fmt.Fprintf(sb, "  %s = zext i1 %s to i8\n", w, b)
		fmt.Fprintf(sb, "  store %si8 %s, ptr %s\n", q, w, ptr)
	case sem.TyChar:
		c := e.irb(sb).Call(rtCharVal, vptr(v)).Text
		fmt.Fprintf(sb, "  store %si32 %s, ptr %s\n", q, c, ptr)
	default:
		ct := e.cScalarType(t)
		raw := e.unbox(sb, v, t, ct)
		fmt.Fprintf(sb, "  store %s%s %s, ptr %s\n", q, ct, raw, ptr)
	}
}

// loadScalar extends box's cases to Bool and Char.
func (e *emitter) loadScalar(sb *strings.Builder, t sem.TypeID, ptr string) string {
	return e.loadScalarQ(sb, t, ptr, "")
}

func (e *emitter) loadScalarQ(sb *strings.Builder, t sem.TypeID, ptr, q string) string {
	switch t {
	case sem.TyBool:
		raw := e.newTmp()
		fmt.Fprintf(sb, "  %s = load %si8, ptr %s\n", raw, q, ptr)
		bit := e.newTmp()
		fmt.Fprintf(sb, "  %s = trunc i8 %s to i1\n", bit, raw)
		return e.boolBox(sb, bit)
	case sem.TyChar:
		raw := e.newTmp()
		fmt.Fprintf(sb, "  %s = load %si32, ptr %s\n", raw, q, ptr)
		return e.irb(sb).Call(rtChar, vi32text(raw)).Text
	default:
		ct := e.cScalarType(t)
		raw := e.newTmp()
		fmt.Fprintf(sb, "  %s = load %s%s, ptr %s\n", raw, q, ct, ptr)
		return e.box(sb, raw, t, ct)
	}
}
