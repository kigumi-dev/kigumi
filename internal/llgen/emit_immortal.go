package llgen

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"kigumi/internal/mir"
)

// fnItem is one capture-free function value: the adapter it calls and
// whether rt_callv must build a Future for it.
type fnItem struct {
	sym   string
	async bool
}

// objSlotBytes is the storage an immortal value is built in; RT_OBJ_SLOT
// in runtime/rt.h must agree (TestObjSlotMatchesC).
const objSlotBytes = 64

func objSlotType() string { return fmt.Sprintf("[%d x i8]", objSlotBytes) }

func boolBit(b bool) int {
	if b {
		return 1
	}
	return 0
}

// variant builds an ADT value, from a shared immortal when its content is
// fixed: the descriptor's own slot with no payload, else a reserved one.
func (e *emitter) variant(sb *strings.Builder, in mir.Inst, args []string) string {
	if len(args) == 0 {
		return e.irb(sb).Call(rtNullary, vptr(e.vdesc(in.Ent))).Text
	}
	if key, ok := e.constKey[in.Dst]; ok && len(args) > 0 {
		for i, a := range in.Args {
			if e.constOwned[a] {
				e.irb(sb).Call(rtRelease, vptr(args[i]))
			}
		}
		return e.constSlotName(key)
	}
	arr := e.argArray(sb, args)
	return e.irb(sb).Call(rtVariant, vptr(e.vdesc(in.Ent)), vi64(len(args)), vptr(arr)).Text
}

// closureCtor builds a function value from sym's adapter; with no
// captures it is the constant kg_init_immortals put in a reserved slot.
func (e *emitter) closureCtor(sb *strings.Builder, sym string, async bool, args []string, arr string) string {
	if len(args) == 0 {
		return e.fnItemSlot(sym, async)
	}
	ctor := rtClosure
	if async {
		ctor = rtClosureAsync
	}
	return e.irb(sb).Call(ctor, vptr(sym), vi32(len(args)), vptr(arr)).Text
}

// stdBind is one capture-free std primitive used as a witness: the key
// rt_std names it by, and which of its arguments are only lent.
type stdBind struct {
	slot string
	key  string
	lent int
}

// stdBindSlot reserves the immortal a generic call passes instead of
// building a witness closure per call; cstr runs here so data() pools the key.
func (e *emitter) stdBindSlot(key string, lent int) string {
	id := fmt.Sprintf("%s|%d", key, lent)
	if slot, ok := e.stdBindOf[id]; ok {
		return slot
	}
	slot := fmt.Sprintf("@\".stdbind.%d\"", len(e.stdBinds))
	e.stdBindOf[id] = slot
	e.stdBinds = append(e.stdBinds, stdBind{slot: slot, key: e.cstr(key), lent: lent})
	return slot
}

func (e *emitter) stdBindSlots() string {
	var sb strings.Builder
	for _, b := range e.stdBinds {
		fmt.Fprintf(&sb, "%s = internal global %s zeroinitializer, align 16\n", b.slot, objSlotType())
	}
	return sb.String()
}

func (e *emitter) fnItemSlot(sym string, async bool) string {
	suffix := "fnitem"
	if async {
		suffix = "fnitem.async"
	}
	name := subSymbol(sym, suffix)
	e.fnItems[name] = fnItem{sym: sym, async: async}
	return name
}

// fnItemSlots defines the zeroed storage each capture-free function value
// is built in; RT_OBJ_SLOT in rt.h fixes the size.
func (e *emitter) fnItemSlots() string {
	var sb strings.Builder
	for _, n := range slices.Sorted(maps.Keys(e.fnItems)) {
		fmt.Fprintf(&sb, "%s = internal global %s zeroinitializer, align 16\n", n, objSlotType())
	}
	return sb.String()
}

// initImmortals fills every immortal up front; built on first use, a slot
// could be read half-written by a handler that preempted its constructor.
func (e *emitter) initImmortals() string {
	var sb strings.Builder
	e.tmp = 0
	sb.WriteString("define void @kg_init_immortals() {\n")
	e.irb(&sb).Call(rtInitNullary, vi32(e.typeCount()), vptr("@rt_types"))
	for _, n := range slices.Sorted(maps.Keys(e.fnItems)) {
		it := e.fnItems[n]
		e.irb(&sb).Call(rtFnItemInit, vptr(n), vptr(it.sym), vi32(boolBit(it.async)))
	}
	for _, b := range e.stdBinds {
		e.irb(&sb).Call(rtStdItemInit, vptr(b.slot), vptr(b.key), vi32(b.lent))
	}
	e.initConstVals(&sb)
	sb.WriteString("  ret void\n}\n\n")
	return sb.String()
}
