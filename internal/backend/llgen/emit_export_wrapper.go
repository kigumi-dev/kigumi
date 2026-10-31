package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

func (e *emitter) exportWrapper(f *mir.Func) string {
	var sb strings.Builder
	sig := e.r.Types.Node(e.r.Fn(f.Ent).Sig)
	var params, vals []string
	for i, pt := range sig.Args {
		params = append(params, fmt.Sprintf("%s %%a%d", e.cType(pt), i))
	}
	ret := e.cType(sig.Elem)
	fmt.Fprintf(&sb, "define%s %s @%s(%s) {\nentry:\n", e.cExt(sig.Elem), ret, e.cName(f.Ent), strings.Join(params, ", "))
	for i, pt := range sig.Args {
		vals = append(vals, e.exportArg(&sb, f, i, pt, fmt.Sprintf("%%a%d", i), e.cType(pt)))
	}
	if out := e.exportCall(&sb, f, vals, sig.Elem, ret); ret != "void" {
		fmt.Fprintf(&sb, "  ret %s %s\n}\n\n", ret, out)
		return sb.String()
	}
	sb.WriteString("  ret void\n}\n\n")
	return sb.String()
}

// exportArg hands C argument i, raw of type pt, to the body: in a register where its signature takes one, else boxed.
func (e *emitter) exportArg(sb *strings.Builder, f *mir.Func, i int, pt sem.TypeID, raw, ct string) string {
	if _, ok := e.ssigs[f].param(i); ok {
		return e.cWiden(sb, raw, pt, ct).render()
	}
	// rt_unit() is immortal, so it stands in here: a trivial export(C) fn
	// can end up as a signal handler, where malloc/free must not reenter.
	if deadParam(f, i) {
		return "ptr " + e.irb(sb).Call(rtUnit).Text
	}
	return "ptr " + e.box(sb, raw, pt, ct)
}

// exportCall calls the body and returns its result as the C type ct, "" for void.
func (e *emitter) exportCall(sb *strings.Builder, f *mir.Func, vals []string, rt sem.TypeID, ct string) string {
	r := e.newTmp()
	if info, ok := e.ssigs[f].ret(); ok {
		fmt.Fprintf(sb, "  %s = call %s %s(%s)\n", r, info.Ty, e.sym(f), strings.Join(vals, ", "))
		return e.cNarrow(sb, Value{info.Ty, r}, rt, ct)
	}
	fmt.Fprintf(sb, "  %s = call ptr %s(%s)\n", r, e.sym(f), strings.Join(vals, ", "))
	out := ""
	if ct != "void" {
		out = e.unbox(sb, r, rt, ct)
	}
	e.irb(sb).Call(rtRelease, vptr(r))
	return out
}

// cExport is an export(C) body: its wrapper passes the register positions of its signature unboxed.
func (e *emitter) cExport(f *mir.Func) bool {
	if e.r.Entity(f.Ent).Kind != sem.EntFn {
		return false
	}
	info := e.r.Fn(f.Ent)
	return info.Export != "" && info.Export != "interrupt" && !info.Naked
}

// deadParam reports whether f's body never reads parameter i: only MIR's own
// scope-exit drop is not a use, and any other reference counts as one.
func deadParam(f *mir.Func, i int) bool {
	if i >= len(f.Params) {
		return false
	}
	id := f.Params[i]
	for _, b := range f.Blocks {
		for _, in := range b.Insts {
			trailing := in.Op == mir.OpDrop || in.Op == mir.OpRelease
			if in.Dst == id && !trailing {
				return false
			}
			for _, a := range in.Args {
				if a == id && !trailing {
					return false
				}
			}
		}
		for _, a := range b.Term.Args {
			if a == id {
				return false
			}
		}
	}
	return true
}
