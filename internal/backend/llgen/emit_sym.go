package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (e *emitter) sym(f *midir.Func) string {
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
func (e *emitter) symName(f *midir.Func) string {
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

func (e *emitter) adapterSym(f *midir.Func) string { return "@\"" + e.symName(f) + "$adapter\"" }

// adapter wraps a function as a closure body taking an environment array
// and an argument array, for function values and dynamic dispatch.
func (e *emitter) adapter(f *midir.Func) string {
	var sb strings.Builder
	ncaps := 0
	if e.r.Entity(f.Ent).Kind == sem.EntClosure {
		ncaps = len(e.r.Closure(f.Ent).Captures)
	}
	fmt.Fprintf(&sb, "define ptr %s(ptr %%env, ptr %%args) {\n", e.adapterSym(f))
	var vals []string
	boxedBody := e.directSig(f) == nil
	slots := paramSlots(midir.BoundTrails(midir.OpClosure), ncaps, len(f.Params))
	for i := range f.Params {
		v := fmt.Sprintf("%%a%d", i)
		if s := slots[i]; s.env && boxedBody && e.p.CaptureLent(f, s.index) {
			// Borrowed in place, so a panic in the callee leaves no reference to give back.
			e.irb(&sb).CallAs(v, rtIterAt, vptr("%env"), vi64(s.index), vi32(1))
		} else {
			e.slotArg(&sb, v, s)
		}
		vals = append(vals, v)
	}
	res := e.callBoxed(&sb, f, vals)
	fmt.Fprintf(&sb, "  ret ptr %s\n}\n\n", res)
	return sb.String()
}

// bindSym names the adapter of a function whose trailing witness
// parameters come from a closure environment (mir.OpBind).
func (e *emitter) bindSym(f *midir.Func) string { return "@\"" + e.symName(f) + "$bind\"" }

// bindAdapter: rt_at's retain on the witness slots offsets invoke's
// release of the argument array, so a lent receiver's refcount stays
// balanced and the caller's reference is untouched.
func (e *emitter) bindAdapter(f *midir.Func) string {
	var sb strings.Builder
	info := e.r.Fn(f.Ent)
	nw := len(info.Witnesses) + len(info.ConstParams)
	fmt.Fprintf(&sb, "define ptr %s(ptr %%env, ptr %%args) {\n", e.bindSym(f))
	var vals []string
	slots := paramSlots(midir.BoundTrails(midir.OpBind), nw, len(f.Params))
	for i := range f.Params {
		v := fmt.Sprintf("%%a%d", i)
		e.slotArg(&sb, v, slots[i])
		vals = append(vals, v)
	}
	fmt.Fprintf(&sb, "  ret ptr %s\n}\n\n", e.callBoxed(&sb, f, vals))
	return sb.String()
}
