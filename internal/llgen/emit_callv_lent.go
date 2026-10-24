package llgen

import (
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// markLent gives a user function value its lent mask, the call arguments after off its parameters only borrow;
// a capture-free value gets it when its immortal is filled.
func (e *emitter) markLent(sb *strings.Builder, in mir.Inst, off int, captureFree bool, v string) string {
	k := e.r.Entity(in.Ent).Kind
	if in.Async || k != sem.EntFn && k != sem.EntClosure {
		return v
	}
	mask := e.lentMask(in.Ent, off)
	if captureFree {
		it := e.fnItems[v]
		it.lent = mask
		e.fnItems[v] = it
		return v
	}
	return e.irb(sb).Call(rtClosureLent, vptr(v), vi32(mask)).Text
}

// lentMask sets bit i when call argument i lands in a parameter of ent that only borrows it: a function's lent
// parameter after the off its env fills, or a closure's `&T` parameter. Bit 31 marks the mask as complete.
func (e *emitter) lentMask(ent sem.EntityID, off int) int {
	mask := 1 << 31
	switch e.r.Entity(ent).Kind {
	case sem.EntFn:
		for i := 0; off+i < e.paramCount(ent) && i < 31; i++ {
			if e.p.ParamLent(ent, off+i) {
				mask |= 1 << i
			}
		}
	case sem.EntClosure:
		for i, prm := range e.r.Closure(ent).Params {
			if i < 31 && e.r.Types.Kind(e.r.Entity(prm).Type) == sem.KRef {
				mask |= 1 << i
			}
		}
	}
	return int(int32(uint32(mask)))
}

func (e *emitter) paramCount(fn sem.EntityID) int {
	info := e.r.Fn(fn)
	n := len(info.Params)
	if info.Recv != sem.RecvNone {
		n++
	}
	return n
}

// callValue calls a function value declaring how the site passes each argument (mir.Inst.LendArgs / GiveArgs), so that
// rt_callv_m can square it with the value's own parameters; a site with nothing to declare keeps the plain call.
func (e *emitter) callValue(sb *strings.Builder, in mir.Inst, args []string, arr string, scalar bool) Value {
	n := vi32(len(args) - 1)
	switch {
	case in.LendArgs == 0 && in.GiveArgs == 0 && scalar:
		return e.irb(sb).Call(rtCallvI64, vptr(args[0]), n, vptr(arr))
	case in.LendArgs == 0 && in.GiveArgs == 0:
		return e.irb(sb).Call(rtCallv, vptr(args[0]), n, vptr(arr))
	}
	lent, given := vi32(int(int32(in.LendArgs))), vi32(int(int32(in.GiveArgs)))
	if scalar {
		return e.irb(sb).Call(rtCallvI64M, vptr(args[0]), n, vptr(arr), lent, given)
	}
	return e.irb(sb).Call(rtCallvM, vptr(args[0]), n, vptr(arr), lent, given)
}
