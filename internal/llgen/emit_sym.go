package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func (e *emitter) sym(f *mir.Func) string {
	// Every cmd/ script has an implicit main; only the entry's is the
	// program's main symbol.
	if f.Name == "main" {
		if f == e.p.Entry {
			return "@\"kigumi.main\""
		}
		return "@\"kigumi.main." + e.entryFile(f.Ent) + "\""
	}
	return "@\"" + e.symName(f) + "\""
}

// entryFile keeps an implicit main's symbol stable across cmd/ scripts.
func (e *emitter) entryFile(ent sem.EntityID) string {
	return e.r.Tree(e.r.Entity(ent).File).File.Name
}

// symName tells overloads apart by position in the overload set, not
// entity number, so symbols don't move when unrelated code changes.
func (e *emitter) symName(f *mir.Func) string {
	if f.Wraps != nil {
		return e.symName(f.Wraps) + "$boxed"
	}
	if e.r.Entity(f.Ent).Kind == sem.EntImplicitMain {
		return "main." + e.entryFile(f.Ent)
	}
	if e.r.Entity(f.Ent).Kind == sem.EntFn {
		// The bare name is the C symbol of an exported function's wrapper.
		if e.r.Fn(f.Ent).Export != "" {
			return f.Name + "$kg"
		}
		if set := e.r.Fn(f.Ent).Set; set != 0 && len(e.r.Overloads[set].Members) > 1 {
			for i, m := range e.r.Overloads[set].Members {
				if m == f.Ent {
					return fmt.Sprintf("%s#%d", f.Name, i)
				}
			}
		}
	}
	return f.Name
}

func (e *emitter) adapterSym(f *mir.Func) string { return "@\"" + e.symName(f) + "$adapter\"" }

// stdAdapterSym lets a bodiless std function be reached through a
// function-pointer slot (a resource drop, a method table) like a Kigumi one.
func (e *emitter) stdAdapterSym(fn sem.EntityID) string {
	return "@\"" + e.stdKey(fn) + "$stdadapter\""
}

func (e *emitter) stdAdapter(fn sem.EntityID) string {
	return e.stdAdapterWith(fn, e.stdAdapterSym(fn), 0, 0)
}

// stdAdapterWith reads the first bound values from the environment (a
// method value's receiver) and the rest from the call's arguments.
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
			e.irb(&sb).CallAs(v, rtAt, vptr("%env"), vi64(i))
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
	finish := func(res string) string {
		for i, v := range vals {
			if i < bound || lent>>i&1 == 0 {
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
	arr := e.argArray(&sb, vals)
	if op, ok := mir.StdOpFor(e.stdKey(fn)); ok {
		e.irb(&sb).CallAs("%r", rtStdId, vi32(int(op)), vi32(n), vptr(arr))
	} else {
		e.irb(&sb).CallAs("%r", rtStd, vptr(e.cstr(e.stdKey(fn))), vi32(n), vptr(arr))
	}
	sb.WriteString(finish("%r"))
	return sb.String()
}

// adapter wraps a function as a closure body taking an environment array
// and an argument array, for function values and dynamic dispatch.
func (e *emitter) adapter(f *mir.Func) string {
	var sb strings.Builder
	ncaps := 0
	if e.r.Entity(f.Ent).Kind == sem.EntClosure {
		ncaps = len(e.r.Closure(f.Ent).Captures)
	}
	fmt.Fprintf(&sb, "define ptr %s(ptr %%env, ptr %%args) {\n", e.adapterSym(f))
	var vals []string
	for i := range f.Params {
		v := fmt.Sprintf("%%a%d", i)
		if i < ncaps {
			e.irb(&sb).CallAs(v, rtAt, vptr("%env"), vi64(i))
		} else {
			e.irb(&sb).CallAs(v, rtAt, vptr("%args"), vi64(i-ncaps))
		}
		vals = append(vals, v)
	}
	res := e.callBoxed(&sb, f, vals)
	if e.directSig(f) == nil {
		for i := 0; i < ncaps && i < len(vals); i++ {
			if e.p.CaptureLent(f, i) {
				e.irb(&sb).Call(rtRelease, vptr(vals[i]))
			}
		}
	}
	fmt.Fprintf(&sb, "  ret ptr %s\n}\n\n", res)
	return sb.String()
}

// bindSym names the adapter of a function whose trailing witness
// parameters come from a closure environment (mir.OpBind).
func (e *emitter) bindSym(f *mir.Func) string { return "@\"" + e.symName(f) + "$bind\"" }

// bindAdapter: rt_at's retain on the witness slots offsets invoke's
// release of the argument array, so a lent receiver's refcount stays
// balanced and the caller's reference is untouched.
func (e *emitter) bindAdapter(f *mir.Func) string {
	var sb strings.Builder
	info := e.r.Fn(f.Ent)
	nw := len(info.Witnesses) + len(info.ConstParams)
	fmt.Fprintf(&sb, "define ptr %s(ptr %%env, ptr %%args) {\n", e.bindSym(f))
	var vals []string
	plain := len(f.Params) - nw
	for i := range f.Params {
		v := fmt.Sprintf("%%a%d", i)
		if i < plain {
			e.irb(&sb).CallAs(v, rtAt, vptr("%args"), vi64(i))
		} else {
			e.irb(&sb).CallAs(v, rtAt, vptr("%env"), vi64(i-plain))
		}
		vals = append(vals, v)
	}
	fmt.Fprintf(&sb, "  ret ptr %s\n}\n\n", e.callBoxed(&sb, f, vals))
	return sb.String()
}
