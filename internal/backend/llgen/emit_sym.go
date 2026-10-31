package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
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

// symName is the function's name, which already carries its overload signature.
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
	}
	return f.Name
}

func (e *emitter) adapterSym(f *mir.Func) string { return "@\"" + e.symName(f) + "$adapter\"" }

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
	boxedBody := e.directSig(f) == nil
	for i := range f.Params {
		v := fmt.Sprintf("%%a%d", i)
		switch {
		case i < ncaps && boxedBody && e.p.CaptureLent(f, i):
			// Borrowed in place, so a panic in the callee leaves no reference to give back.
			e.irb(&sb).CallAs(v, rtIterAt, vptr("%env"), vi64(i), vi32(1))
		case i < ncaps:
			e.irb(&sb).CallAs(v, rtAt, vptr("%env"), vi64(i))
		default:
			e.irb(&sb).CallAs(v, rtAt, vptr("%args"), vi64(i-ncaps))
		}
		vals = append(vals, v)
	}
	res := e.callBoxed(&sb, f, vals)
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
