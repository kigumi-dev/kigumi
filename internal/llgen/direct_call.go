package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) aggArg(sb *strings.Builder, a *aggInfo, l mir.LocalID) string {
	e.requireDirect(l, "argument")
	if byValueAgg(a) {
		return a.Name + " " + e.aggValue(sb, a, aggPtr(l))
	}
	return "ptr " + aggPtr(l)
}

func (e *emitter) directCall(sb *strings.Builder, f *mir.Func, in mir.Inst, fn *mir.Func, s *aggSig) {
	ss := e.ssigs[fn]
	args, lent, out := e.directCallArgs(sb, in, s, ss)
	if r, ok := ss.ret(); ok {
		e.nativeCallInto(sb, in.Dst, e.sym(fn), args, r)
	} else {
		var ret *aggInfo
		if s != nil {
			ret = s.Ret
		}
		e.callInto(sb, in.Dst, e.sym(fn), args, ret, out)
	}
	for i, v := range lent {
		if v != "" {
			e.unslot(sb, in.Args[i])
		}
	}
	e.releaseBoxed(sb, lent)
}

func (e *emitter) retSlot(sb *strings.Builder, s *aggSig) (args []string, out string) {
	if s != nil && s.Ret != nil && !byValueAgg(s.Ret) {
		out = e.stackSlot(sb, s.Ret.Name)
		args = append(args, "ptr "+out)
	}
	return args, out
}

func (e *emitter) callInto(sb *strings.Builder, dst mir.LocalID, sym string, args []string, ret *aggInfo, out string) {
	switch {
	case ret == nil:
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = call ptr %s(%s)\n", t, sym, strings.Join(args, ", "))
		e.storeMaybeScalar(sb, dst, t)
	case byValueAgg(ret):
		v := e.newTmp()
		fmt.Fprintf(sb, "  %s = call %s %s(%s)\n", v, ret.Name, sym, strings.Join(args, ", "))
		fmt.Fprintf(sb, "  store %s %s, ptr %s\n", ret.Name, v, aggPtr(dst))
	default:
		fmt.Fprintf(sb, "  call void %s(%s)\n", sym, strings.Join(args, ", "))
		copyAgg(sb, ret, aggPtr(dst), out)
	}
}

// lent names the box a crossing made for a position the callee only
// borrows: nothing else owns it, so the call site gives it back.
func (e *emitter) directCallArgs(sb *strings.Builder, in mir.Inst, s *aggSig, ss *scalarSig) (args, lent []string, out string) {
	args, out = e.retSlot(sb, s)
	lent = make([]string, len(in.Args))
	for i, l := range in.Args {
		if info, ok := ss.param(i); ok {
			args = append(args, e.nativeArg(sb, in, i, l, info))
			continue
		}
		if a := s.param(i); a != nil {
			args = append(args, e.aggArg(sb, a, l))
			continue
		}
		if e.isScalar(l) {
			args = append(args, "ptr "+e.scalarBoxOnDemand(sb, l))
			continue
		}
		v := e.load(sb, l)
		if e.boxTmp[l] && !e.p.BoxConsumed(in, i) {
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
	e.requireDirect(l, "result")
	if byValueAgg(a) {
		fmt.Fprintf(sb, "  ret %s %s\n", a.Name, e.aggValue(sb, a, aggPtr(l)))
		return
	}
	copyAgg(sb, a, "%sret", aggPtr(l))
	sb.WriteString("  ret void\n")
}

// A crossing is an explicit MIR op, so a position that takes an aggregate never sees a box here.
func (e *emitter) requireDirect(l mir.LocalID, what string) {
	if !e.isDirect(l) {
		panic(fmt.Sprintf("llgen: the %s %%l%d reaches a direct position boxed", what, l))
	}
}
