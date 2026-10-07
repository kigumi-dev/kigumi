package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

// PlanDirect has already put the argument in the form this position
// takes, so no crossing is left to make here.
func (e *emitter) aggArg(sb *strings.Builder, a *aggInfo, l mir.LocalID) string {
	v := e.loadAgg(sb, a, l)
	if byValueAgg(a) {
		return a.Name + " " + v
	}
	slot := e.stackSlot(sb, a.Name)
	fmt.Fprintf(sb, "  store %s %s, ptr %s\n", a.Name, v, slot)
	return "ptr " + slot
}

func (e *emitter) directCall(sb *strings.Builder, f *mir.Func, in mir.Inst, fn *mir.Func, s *aggSig) {
	args, lent, out := e.directCallArgs(sb, in, s)
	if s.Ret == nil {
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = call ptr %s(%s)\n", t, e.sym(fn), strings.Join(args, ", "))
		e.releaseBoxed(sb, lent)
		e.storeMaybeScalar(sb, in.Dst, t)
		return
	}
	v := out
	if byValueAgg(s.Ret) {
		v = e.newTmp()
		fmt.Fprintf(sb, "  %s = call %s %s(%s)\n", v, s.Ret.Name, e.sym(fn), strings.Join(args, ", "))
	} else {
		fmt.Fprintf(sb, "  call void %s(%s)\n", e.sym(fn), strings.Join(args, ", "))
		v = e.newTmp()
		fmt.Fprintf(sb, "  %s = load %s, ptr %s\n", v, s.Ret.Name, out)
	}
	e.releaseBoxed(sb, lent)
	e.storeAgg(sb, e.direct[in.Dst], in.Dst, v)
}

// lent names the box a crossing made for a position the callee only
// borrows: nothing else owns it, so the call site gives it back.
func (e *emitter) directCallArgs(sb *strings.Builder, in mir.Inst, s *aggSig) (args, lent []string, out string) {
	if s.Ret != nil && !byValueAgg(s.Ret) {
		out = e.stackSlot(sb, s.Ret.Name)
		args = append(args, "ptr "+out)
	}
	lent = make([]string, len(in.Args))
	for i, l := range in.Args {
		if a := s.param(i); a != nil {
			args = append(args, e.aggArg(sb, a, l))
			continue
		}
		if e.isScalar(l) {
			args = append(args, "ptr "+e.scalarBoxOnDemand(sb, l))
			continue
		}
		v := e.load(sb, l)
		if e.boxTmp[l] && !e.aggConsumes(in, i) {
			lent[i] = v
		}
		args = append(args, "ptr "+v)
	}
	return args, lent, out
}

func (e *emitter) releaseBoxed(sb *strings.Builder, ptrs []string) {
	for _, p := range ptrs {
		if p != "" {
			e.irb(sb).Call(rtRelease, vptr(p))
		}
	}
}

func (e *emitter) returnAgg(sb *strings.Builder, a *aggInfo, l mir.LocalID) {
	v := e.loadAgg(sb, a, l)
	if byValueAgg(a) {
		fmt.Fprintf(sb, "  ret %s %s\n", a.Name, v)
		return
	}
	fmt.Fprintf(sb, "  store %s %s, ptr %%sret\n  ret void\n", a.Name, v)
}
