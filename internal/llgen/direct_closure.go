package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) envOf(f *mir.Func) (*aggInfo, bool) {
	if !e.p.EnvABI(f) {
		return nil, false
	}
	return e.agg(e.r.Types.Closure(f.Ent))
}

func (e *emitter) closureCall(sb *strings.Builder, in mir.Inst, a *aggInfo) {
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

func (e *emitter) envAdapter(f *mir.Func, env *aggInfo) string {
	e.aggHelpers(env)
	s := e.directSig(f)
	var sb strings.Builder
	fmt.Fprintf(&sb, "define ptr %s(ptr %%env, ptr %%args) {\nentry:\n", e.adapterSym(f))
	slot := e.helperSlot(&sb, env)
	e.unboxLeaves(&sb, env, slot, func(i int) string {
		return fmt.Sprintf("call ptr @rt_at(ptr %%env, i64 %d)", i)
	})
	var args []string
	out := ""
	if s.Ret != nil {
		out = e.helperSlot(&sb, s.Ret)
		if !byValueAgg(s.Ret) {
			args = append(args, "ptr "+out)
		}
	}
	args = append(args, "ptr "+slot)
	for i := len(env.Fields); i < len(f.Params); i++ {
		a := e.newTmp()
		e.irb(&sb).CallAs(a, rtAt, vptr("%args"), vi64(i-len(env.Fields)))
		p := s.param(i)
		if p == nil {
			args = append(args, "ptr "+a)
			continue
		}
		u := e.helperSlot(&sb, p)
		e.unboxAgg(&sb, p, u, a)
		e.irb(&sb).Call(rtRelease, vptr(a))
		if byValueAgg(p) {
			args = append(args, p.Name+" "+e.aggValue(&sb, p, u))
		} else {
			args = append(args, "ptr "+u)
		}
	}
	r := e.newTmp()
	switch {
	case s.Ret == nil:
		fmt.Fprintf(&sb, "  %s = call ptr %s(%s)\n", r, e.sym(f), strings.Join(args, ", "))
	case byValueAgg(s.Ret):
		fmt.Fprintf(&sb, "  %s = call %s %s(%s)\n  store %s %s, ptr %s\n", r, s.Ret.Name, e.sym(f), strings.Join(args, ", "), s.Ret.Name, r, out)
		r = e.boxAgg(&sb, s.Ret, out)
	default:
		fmt.Fprintf(&sb, "  call void %s(%s)\n", e.sym(f), strings.Join(args, ", "))
		r = e.boxAgg(&sb, s.Ret, out)
	}
	fmt.Fprintf(&sb, "  ret ptr %s\n}\n\n", r)
	return sb.String()
}

// Not stackSlot: that hoists into the function being emitted, not into this helper.
func (e *emitter) helperSlot(sb *strings.Builder, a *aggInfo) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = alloca %s\n", t, a.Name)
	return t
}

// rt_closure retains each capture, so the boxes made here are released after.
func (e *emitter) cloBoxFn(a *aggInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal ptr %s(ptr %%v) {\nentry:\n", e.aggBoxSym(a))
	arr, boxes := e.boxLeaves(&sb, a, "%v")
	fmt.Fprintf(&sb, "  %%r = call ptr @rt_closure(ptr %s, i32 %d, ptr %s)\n", e.useAdapter(a.Clo), len(a.Fields), arr)
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
