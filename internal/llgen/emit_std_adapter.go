package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// stdAdapterSym lets a bodiless std function be reached through a
// function-pointer slot (a resource drop, a method table) like a Kigumi one.
func (e *emitter) stdAdapterSym(fn sem.EntityID) string {
	return "@\"" + e.stdKey(fn) + "$stdadapter\""
}

func (e *emitter) stdAdapter(fn sem.EntityID) string {
	return e.stdAdapterWith(fn, e.stdAdapterSym(fn), 0, 0)
}

// stdAdapterWith borrows the first bound values from the environment (a
// method value's receiver) and takes the rest from the call's arguments.
func (e *emitter) stdAdapterWith(fn sem.EntityID, sym string, bound int, value sem.TypeID) string {
	defer func(h bool) { e.hoisting = h }(e.hoisting)
	e.hoisting = false
	var sb strings.Builder
	fmt.Fprintf(&sb, "define ptr %s(ptr %%env, ptr %%args) {\n", sym)
	n := len(e.r.Types.Node(e.r.Fn(fn).Sig).Args)
	if e.r.Fn(fn).Recv != sem.RecvNone {
		n++
	}
	var vals []string
	for i := range n {
		v := fmt.Sprintf("%%a%d", i)
		if i < bound {
			e.irb(&sb).CallAs(v, rtIterAt, vptr("%env"), vi64(i), vi32(1))
		} else {
			e.irb(&sb).CallAs(v, rtAt, vptr("%args"), vi64(i-bound))
		}
		vals = append(vals, v)
	}
	// A primitive llgen expands at call sites (a CValue's drop) is
	// expanded here too, so the runtime never sees its key.
	sig := e.r.Types.Node(e.r.Fn(fn).Sig)
	var types []sem.TypeID
	if e.r.Fn(fn).Recv != sem.RecvNone {
		types = append(types, 0)
	}
	result := sig.Elem
	if vt := e.r.Types.Node(value); value != 0 && vt.Kind == sem.KFn && e.r.ConcreteArg(value) {
		types = append(types, make([]sem.TypeID, bound)...)
		types = append(types, vt.Args...)
		result = vt.Elem
	} else {
		types = append(types, sig.Args...)
	}
	// The primitive only lends, so the adapter returns the references rt_at took, except for borrowed parameters.
	lent := mir.LentMask(e.r, fn)
	var arr string
	guarded := e.catch && e.stdArgsOwned(lent, bound, n)
	if guarded {
		arr = e.argArray(&sb, vals)
		sb.WriteString("  %urec = alloca [10 x i64], align 8\n")
		e.irb(&sb).Call(rtUnwindEnterStd, vptr("%urec"), vptr(arr), vi64(n), vi64(lent|(1<<bound-1)))
	}
	finish := func(res string) string {
		if guarded {
			e.irb(&sb).Call(rtUnwindLeave, vptr("%urec"))
		}
		for i, v := range vals {
			if i >= bound && lent>>i&1 == 0 {
				e.irb(&sb).Call(rtRelease, vptr(v))
			}
		}
		return fmt.Sprintf("  ret ptr %s\n}\n\n", res)
	}
	if e.stdKey(fn) == "std/error.as" && len(vals) == 1 {
		res := e.errorAsCall(&sb, vals[0], result)
		sb.WriteString(finish(res))
		return sb.String()
	}
	if res, ok := e.ffiRecordCall(&sb, e.stdKey(fn), vals, types, result); ok {
		sb.WriteString(finish(res))
		return sb.String()
	}
	if info := e.r.Fn(fn); info.Owner != 0 && len(vals) == 1 {
		if res, ok := e.convertCall(&sb, fn, e.stdKey(fn), vals, e.r.Entity(info.Owner).Type, sig.Elem); ok {
			sb.WriteString(finish(res))
			return sb.String()
		}
	}
	if arr == "" {
		arr = e.argArray(&sb, vals)
	}
	if op, ok := mir.StdOpFor(e.stdKey(fn)); ok {
		e.irb(&sb).CallAs("%r", rtStdId, vi32(int(op)), vi32(n), vptr(arr))
	} else {
		e.irb(&sb).CallAs("%r", rtStd, vptr(e.cstr(e.stdKey(fn))), vi32(n), vptr(arr))
	}
	sb.WriteString(finish("%r"))
	return sb.String()
}

// stdArgsOwned reports that the adapter holds a reference of its own to some argument, which a panic would leave behind.
func (e *emitter) stdArgsOwned(lent, bound, n int) bool {
	for i := bound; i < n; i++ {
		if lent>>i&1 == 0 {
			return true
		}
	}
	return false
}
