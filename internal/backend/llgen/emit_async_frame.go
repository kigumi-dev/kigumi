package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/mir"
)

func (e *emitter) frameLocal(sb *strings.Builder, l mir.LocalID) {
	if e.slotAddr(sb, l) {
		fmt.Fprintf(sb, "  store ptr null, ptr %%l%d\n", l)
		return
	}
	if a, ok := e.direct[l]; ok {
		fmt.Fprintf(sb, "  %%l%d = alloca %s\n", l, a.Name)
		zeroAgg(sb, a, aggPtr(l))
		return
	}
	if info, ok := e.scalar[l]; ok {
		fmt.Fprintf(sb, "  %%l%d = alloca %s\n", l, info.Ty)
		fmt.Fprintf(sb, "  store %s %s, ptr %%l%d\n", info.Ty, zeroOf(info.Ty), l)
		return
	}
	fmt.Fprintf(sb, "  %%l%d = alloca ptr\n", l)
	fmt.Fprintf(sb, "  store ptr null, ptr %%l%d\n", l)
}

func (e *emitter) checkSlot(f *mir.Func, l mir.LocalID) {
	a, ok := e.direct[l]
	if !ok {
		return
	}
	if a.Size > e.p.SlotBytes(f, l) {
		panic(fmt.Sprintf("llgen: %s: %s needs %d bytes, its frame slot has %d", f.Name, a.Name, a.Size, e.p.SlotBytes(f, l)))
	}
}

// Inline bytes and raw scalars own no reference.
func (e *emitter) carrierSlots(locals []mir.LocalID) []mir.LocalID {
	var out []mir.LocalID
	for _, l := range locals {
		if !e.isDirect(l) && !e.isScalar(l) {
			out = append(out, l)
		}
	}
	return out
}

// zeroOf initializes a scalar local's alloca the way every other local
// starts out null.
func zeroOf(t Type) string {
	switch t {
	case TDouble, TFloat:
		return "0.0"
	case TCPtr:
		return "null"
	}
	return "0"
}
