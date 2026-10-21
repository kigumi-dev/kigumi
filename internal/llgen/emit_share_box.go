package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) share(sb *strings.Builder, f *mir.Func, in mir.Inst, v string) string {
	switch {
	case in.Copy && e.scalarBoxed(f.Locals[in.Args[0]].Type):
		// rt_copy retains a scalar box, and a retain says so to an IR audit.
		return e.irb(sb).Call(rtRetain, vptr(v)).Text
	case in.Copy && in.IfWritable:
		return e.irb(sb).Call(rtCopyMut, vptr(v)).Text
	case in.Copy:
		return e.irb(sb).Call(rtCopy, vptr(v)).Text
	}
	return e.irb(sb).Call(rtRetain, vptr(v)).Text
}

// lendBox builds an `&Interface` view in this frame: no allocation, and no count of v taken.
// An async body builds it in the Future's frame, since a callee may hold it across a Pending.
func (e *emitter) lendBox(sb *strings.Builder, f *mir.Func, in mir.Inst, v string) string {
	var slot string
	if off, ok := f.ViewSlot[in.Dst]; ok {
		slot = e.newTmp()
		fmt.Fprintf(sb, "  %s = getelementptr ptr, ptr %%slots, i64 %d\n", slot, off)
	} else {
		slot = e.stackSlot(sb, objSlotType()+", align 16")
	}
	iface := e.r.Types.Node(f.Locals[in.Dst].Type).Elem
	e.irb(sb).Call(rtBoxLent, vptr(slot), vptr(v), vptr(e.descOfType(in.Type)), vptr(e.vtable(in.Type, iface, e.r.Entity(f.Ent).Pkg)))
	return slot
}
