package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

func (e *emitter) envOf(f *midir.Func) (*aggInfo, bool) {
	if !e.p.EnvABI(f) {
		return nil, false
	}
	return e.agg(e.r.Types.Closure(f.Ent))
}

func (e *emitter) closureCall(sb *strings.Builder, in midir.Inst, a *aggInfo) {
	s := e.directSig(a.Clo)
	args, out := e.retSlot(sb, s)
	args = append(args, "ptr "+aggPtr(in.Args[0]))
	for k, l := range in.Args[1:] {
		if p := s.param(len(a.Fields) + k); p != nil {
			args = append(args, e.aggArg(sb, p, l))
			continue
		}
		args = append(args, "ptr "+e.load(sb, l))
	}
	e.callInto(sb, in.Dst, e.sym(a.Clo), args, s.Ret, out)
}

// rt_closure retains each capture, so the boxes made here are released after.
func (e *emitter) cloBoxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal ptr %s(ptr %%v) {\nentry:\n", e.aggBoxSym(a))
	arr, boxes := e.boxLeaves(&sb, a, "%v")
	fmt.Fprintf(&sb, "  %%r = call ptr @rt_closure(ptr %s, i32 %d, ptr %s)\n", e.useAdapter(a.Clo), len(a.Fields), arr)
	fmt.Fprintf(&sb, "  call ptr @rt_closure_lent(ptr %%r, i32 %d)\n", e.lentMask(a.Clo.Ent, len(a.Fields)))
	for _, b := range boxes {
		fmt.Fprintf(&sb, "  call void @rt_release(ptr %s)\n", b)
	}
	sb.WriteString("  ret ptr %r\n}\n\n")
	return sb.String()
}

func (e *emitter) cloUnboxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal void %s(ptr %%d, ptr %%b) {\nentry:\n", e.aggUnboxSym(a))
	sb.WriteString("  %env = call ptr @rt_closure_env(ptr %b)\n")
	e.unboxLeaves(&sb, a, "%d", func(i int) string {
		return fmt.Sprintf("call ptr @rt_at(ptr %%env, i64 %d)", i)
	})
	sb.WriteString("  ret void\n}\n\n")
	return sb.String()
}
