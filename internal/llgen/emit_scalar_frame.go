package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

// scalarFrameBox boxes a scalar for a reader that never keeps it, in an immortal the frame holds.
func (e *emitter) scalarFrameBox(sb *strings.Builder, l mir.LocalID) string {
	if slot, ok := e.litSlot(l); ok {
		return slot
	}
	info := e.scalar[l]
	v := e.loadNative(sb, l)
	num, f, kind := vi64(0), vdouble("0.0"), kInt
	switch info.Ty {
	case TI1:
		return e.boolBox(sb, v.Text)
	case TDouble:
		f, kind = v, kFloat
	case TI32:
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = zext i32 %s to i64\n", t, v.Text)
		num, kind = vi64text(t), kChar
	default:
		num = v
	}
	slot := e.stackSlot(sb, objSlotType()+", align 16")
	if info.Ty == TI128 {
		lo, hi := e.splitWide(sb, v.Text)
		e.irb(sb).Call(rtConstInt128, vptr(slot), vi32(info.Nk), lo, hi)
		return slot
	}
	e.irb(sb).Call(rtConstNum, vptr(slot), vi32(kind), vi32(info.Nk), num, f)
	return slot
}
