package llgen

import (
	"maps"
	"slices"
	"strings"

	opdef "kigumi/internal/compiler/mir/ops"
)

// stdUse is what the emitted code asks of rt_std_id: the ops it names by id
// and whether it names a std function by key (rt_std, rt_std_bind, rt_stditem_init).
type stdUse struct {
	ops     map[opdef.StdOpId]bool
	dynamic bool
}

func (u *stdUse) add(op opdef.StdOpId) {
	if u.ops == nil {
		u.ops = map[opdef.StdOpId]bool{}
	}
	u.ops[op] = true
}

// stdIdCall emits rt_std_id for op into dst ("" takes a fresh register).
func (e *emitter) stdIdCall(sb *strings.Builder, dst string, op opdef.StdOpId, n int, arr string) string {
	e.std.add(op)
	if e.boxes != nil && opdef.Registry.Ops[op].ScalarBox != nil {
		e.boxes.primitives[e.at] = op
	}
	// Pointer-valued runtime primitives return an owned box even when the caller immediately unboxes it.
	if e.boxes != nil && opdef.Registry.Ops[op].Desc.Result == opdef.ClassSet(1<<opdef.ClassPtr) {
		e.boxes.made[boxSite{e.at, -1}] = true
	}
	if dst == "" {
		return e.irb(sb).Call(rtStdId, vi32(int(op)), vi32(n), vptr(arr)).Text
	}
	e.irb(sb).CallAs(dst, rtStdId, vi32(int(op)), vi32(n), vptr(arr))
	return dst
}

// stdKeyStr is the C string a keyed runtime call resolves at run time; the op it names stays compiled in.
func (e *emitter) stdKeyStr(key string) string {
	e.std.dynamic = true
	if op, ok := opdef.StdOpFor(key); ok {
		e.std.add(op)
	}
	return e.cstr(key)
}

// macros are the -D names that compile exactly the runtime's cases this use reaches.
func (u stdUse) macros() []string {
	out := []string{"KIGUMI_OP_SELECT"}
	if u.dynamic {
		out = append(out, "KIGUMI_STD_DYNAMIC")
	}
	for _, op := range slices.Sorted(maps.Keys(u.ops)) {
		if entry := opdef.Registry.Ops[op]; !entry.Retired {
			out = append(out, OpMacro(entry.Name))
		}
	}
	return out
}
